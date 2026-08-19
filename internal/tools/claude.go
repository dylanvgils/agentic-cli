package tools

import (
	"net/http"
	"os"
	"path"
	"path/filepath"

	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/mount"
)

// claudeLatestVersionURL mirrors the version check performed by claude.ai/install.sh
// before it picks a binary to download - install.sh always resolves this URL
// regardless of any stable/latest/version target, so checking it here matches
// exactly what the install step below actually fetches.
const claudeLatestVersionURL = "https://downloads.claude.ai/claude-code-releases/latest"

// claudeInstructionsContainerPath is the container-side mount target for a per-run instructions snapshot.
const claudeInstructionsContainerPath = "$CONTAINER_HOME/.claude/CLAUDE.md"

// claudeAllowedHosts is the baseline egress allowlist for Claude Code. Package
// registries or other hosts are added by the user via allowed_hosts.
var claudeAllowedHosts = []string{
	".anthropic.com", // Claude API and telemetry subdomains (e.g. api.anthropic.com, statsig.anthropic.com)
	".claude.ai",     // installer and asset downloads (e.g. downloads.claude.ai)
	".claude.com",    // OAuth/login flow
}

func claudeTmpfsMounts() []string {
	return []string{
		mount.TmpfsMount("/tmp", mount.TmpfsOptions{Exec: true, Size: "1g"}),
	}
}

func claudeMounts() []string {
	return []string{
		mount.VolumeMount("$PWD", "/workspace"),
		mount.VolumeMount("$TOOL_HOME/claude/data", "$CONTAINER_HOME/.claude"),
		mount.VolumeMount("$TOOL_HOME/claude/.claude.json", "$CONTAINER_HOME/.claude.json"),
	}
}

// claudeMarketplaceMount mounts a synced marketplace clone read-only outside Claude's own state tree.
func claudeMarketplaceMount(name, url string) string {
	return mount.VolumeMount(
		path.Join("$TOOL_HOME", marketplace.MarketplacesDirName, marketplace.CloneDirName(url)),
		path.Join("$CONTAINER_HOME", marketplace.MarketplacesDirName, name),
		mount.VolumeOptions{ReadOnly: true},
	)
}

func claudeStage(prevStage string) df.Stage {
	return df.NewStage(df.From{Image: prevStage, As: "tool"}).
		Add(df.Shell{Cmd: []string{"/bin/bash", "-o", "pipefail", "-c"}}).
		Add(createContainerUser("claude")...).
		Add(df.Heredoc{
			Dest: "/usr/local/bin/entrypoint.sh",
			Blocks: []df.Block{
				{Lines: []string{"#!/usr/bin/env bash", "set -euo pipefail"}},
				{
					Comment: "Register AGENTIC_MARKETPLACES into the Claude container",
					Lines: []string{
						`marketplaces_dir="$HOME/` + marketplace.MarketplacesDirName + `"`,
						`if [[ -n "${AGENTIC_MARKETPLACES:-}" ]]; then`,
						`  IFS=',' read -ra marketplace_names <<< "$AGENTIC_MARKETPLACES"`,
						`  for name in "${marketplace_names[@]}"; do`,
						`    dir="$marketplaces_dir/$name"`,
						`    [[ -d "$dir" ]] || continue`,
						`    claude plugin marketplace add "$dir" --scope user || echo "warning: failed to register marketplace $name" >&2`,
						"  done",
						"fi",
					},
				},
				{
					Comment: "Deregister marketplaces this container no longer mounts, e.g. removed from .agenticrc.toml",
					Lines: []string{
						`while IFS=$'\t' read -r name loc; do`,
						`  [[ -n "$name" && "$(dirname -- "$loc")" == "$marketplaces_dir" && ! -d "$loc" ]] || continue`,
						`  claude plugin marketplace remove "$name" --scope user || echo "warning: failed to deregister marketplace $name" >&2`,
						`done < <(claude plugin marketplace list --json 2>/dev/null | jq -r '.[] | select(.source=="directory") | [.name, .installLocation] | @tsv' 2>/dev/null)`,
					},
				},
				{Lines: []string{`exec claude "$@"`}},
			},
		}).
		Add(df.User{Name: "claude"}).
		Add(df.Env{Key: "PATH", Value: "/home/claude/.local/bin:${PATH}"}).
		Add(df.Run{Blocks: []df.Block{
			{Lines: []string{"curl -fsSL https://claude.ai/install.sh | bash"}},
			{Lines: []string{`mkdir -p "/home/claude/.claude"`}},
		}}).
		Add(df.Heredoc{
			Dest:  "/usr/local/bin/" + versionScript("claude"),
			Lines: []string{"#!/bin/sh", "claude --version"},
		}).
		Add(df.Env{Key: "TOOL_HOME", Value: "/home/claude"}).
		Add(df.Workdir{Path: "/workspace"}).
		Add(df.Entrypoint{Cmd: []string{"/usr/local/bin/entrypoint.sh"}}).
		Build()
}

// claudeLatestVersion fetches the latest Claude Code version available upstream.
func claudeLatestVersion() (string, error) {
	return fetchTextVersion(claudeLatestVersionURL, http.DefaultClient)
}

func setupClaude(toolHome string) error {
	if err := os.MkdirAll(filepath.Join(toolHome, "claude", "data"), 0o750); err != nil {
		return err
	}

	path := filepath.Join(toolHome, "claude", ".claude.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.WriteFile(path, []byte("{}"), 0o640)
	}

	return nil
}

// claudeInstructionsHostPath is Claude Code's global CLAUDE.md (~/.claude/CLAUDE.md).
func claudeInstructionsHostPath(toolHome string) string {
	return filepath.Join(toolHome, "claude", "data", "CLAUDE.md")
}

// writeClaudeInstructions writes content to Claude Code's global CLAUDE.md.
func writeClaudeInstructions(toolHome, content string) error {
	return writeManagedInstructions(claudeInstructionsHostPath(toolHome), content)
}
