package tools

import (
	"os"
	"path"
	"path/filepath"

	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/mount"
)

// copilotReleaseRepo is the GitHub repo the install script (gh.io/copilot-install) pulls releases from.
const copilotReleaseRepo = "github/copilot-cli"

// copilotInstructionsContainerPath is the container-side mount target for a per-run instructions snapshot.
const copilotInstructionsContainerPath = "$CONTAINER_HOME/.copilot/copilot-instructions.md"

// copilotAllowedHosts is the baseline egress allowlist for GitHub Copilot CLI; users add more via allowed_hosts.
var copilotAllowedHosts = []string{
	".githubcopilot.com", // Copilot API and subdomains (e.g. api.githubcopilot.com, telemetry.githubcopilot.com)
	"api.github.com",     // GitHub API used for authentication
}

func copilotTmpfsMounts() []string {
	return []string{
		mount.TmpfsMount("/tmp", mount.TmpfsOptions{Exec: true, Size: "1g"}),
		mount.TmpfsMount("$CONTAINER_HOME/.cache", mount.TmpfsOptions{Exec: true, Size: "1g"}),
	}
}

func copilotMounts() []string {
	return []string{
		mount.VolumeMount("$PWD", mount.WorkspaceContainerPath),
		mount.VolumeMount(path.Join("$TOOL_HOME", ToolsDirName, "copilot"), "$CONTAINER_HOME/.copilot"),
	}
}

// copilotMarketplaceMount mounts a synced marketplace clone read-only; entrypoint.sh registers it.
func copilotMarketplaceMount(name, url string) string {
	return mount.VolumeMount(
		path.Join("$TOOL_HOME", marketplace.MarketplacesDirName, marketplace.CloneDirName(url)),
		path.Join("$CONTAINER_HOME", marketplace.MarketplacesDirName, name),
		mount.VolumeOptions{ReadOnly: true},
	)
}

func copilotStage(prevStage string) df.Stage {
	return df.NewStage(df.From{Image: prevStage, As: "tool"}).
		Add(df.Shell{Cmd: []string{"/bin/bash", "-o", "pipefail", "-c"}}).
		Add(createContainerUser("copilot")...).
		Add(df.Run{Command: "curl -fsSL https://gh.io/copilot-install | bash"}).
		Add(df.Heredoc{
			Dest:  "/usr/local/bin/" + versionScript("copilot"),
			Lines: []string{"#!/bin/sh", "copilot --version"},
		}).
		Add(df.Heredoc{
			Dest: "/usr/local/bin/entrypoint.sh",
			Blocks: []df.Block{
				{Lines: []string{"#!/usr/bin/env bash", "set -euo pipefail"}},
				{
					Comment: "Set GITHUB_TOKEN if mounted in container",
					Lines: []string{
						"if [[ -f /run/secrets/copilot_token ]]; then",
						`  export GITHUB_TOKEN="$(cat /run/secrets/copilot_token)"`,
						"fi",
					},
				},
				{
					Lines: []string{
						"# Register AGENTIC_MARKETPLACES into Copilot",
						"# Not idempotent, so skip marketplaces already registered (e.g. from a previous run against",
						"# the same persisted $HOME/.copilot mount). Matched by local path rather than name, since",
						"# copilot's registered name is derived from the manifest and may differ from ours - see the",
						"# fragility note below.",
						`marketplaces_dir="$HOME/` + marketplace.MarketplacesDirName + `"`,
						`if [[ -n "${AGENTIC_MARKETPLACES:-}" ]]; then`,
						`  registered_locs="$(copilot plugin marketplace list 2>/dev/null | awk '{i=index($0,"(Local: "); if (!i) next; loc=substr($0,i+8); sub(/\)$/,"",loc); print loc}')"`,
						`  IFS=',' read -ra marketplace_names <<< "$AGENTIC_MARKETPLACES"`,
						`  for name in "${marketplace_names[@]}"; do`,
						`    dir="$marketplaces_dir/$name"`,
						`    [[ -d "$dir" ]] || continue`,
						`    grep -qxF "$dir" <<< "$registered_locs" && continue`,
						`    copilot plugin marketplace add "$dir" || echo "warning: failed to register marketplace $name" >&2`,
						"  done",
						"fi",
					},
				},
				{
					Lines: []string{
						"# Deregister marketplaces this container no longer mounts, e.g. removed from .agenticrc.toml",
						"# copilot plugin marketplace list has no --json output, so parse its \"NAME (Local: PATH)\" lines instead.",
						"# Fragile: this text format is not a stable contract. Simplify (or drop the path-matching) once",
						"# copilot ships machine-readable list output.",
						`while IFS=$'\t' read -r name loc; do`,
						`  [[ -n "$name" && "$(dirname -- "$loc")" == "$marketplaces_dir" && ! -d "$loc" ]] || continue`,
						`  copilot plugin marketplace remove --force "$name" || echo "warning: failed to deregister marketplace $name" >&2`,
						`done < <(copilot plugin marketplace list 2>/dev/null | awk '{i=index($0,"(Local: "); if (!i) next; name=$0; sub(/ \(Local:.*/,"",name); sub(/^.*[ \t]/,"",name); loc=substr($0,i+8); sub(/\)$/,"",loc); printf "%s\t%s\n",name,loc}')`,
					},
				},
				{Lines: []string{`exec copilot "$@"`}},
			},
		}).
		Add(df.Run{Command: "mkdir -p /home/copilot/.copilot"}).
		Add(df.User{Name: "copilot"}).
		Add(df.Env{Key: "TOOL_HOME", Value: "/home/copilot"}).
		Add(df.Env{Key: "COPILOT_AUTO_UPDATE", Value: "false"}).
		Add(df.Workdir{Path: mount.WorkspaceContainerPath}).
		Add(df.Entrypoint{Cmd: []string{"/usr/local/bin/entrypoint.sh"}}).
		Build()
}

// copilotLatestVersion fetches the latest GitHub Copilot CLI version available upstream.
func copilotLatestVersion() (string, error) {
	return latestGithubTag(copilotReleaseRepo)
}

func setupCopilot(toolHome string) error {
	return os.MkdirAll(filepath.Join(toolHome, ToolsDirName, "copilot"), 0o750)
}

// copilotInstructionsHostPath is Copilot CLI's global instructions file (~/.copilot/copilot-instructions.md).
func copilotInstructionsHostPath(toolHome string) string {
	return filepath.Join(toolHome, ToolsDirName, "copilot", "copilot-instructions.md")
}

// writeCopilotInstructions writes content to Copilot CLI's global instructions file.
func writeCopilotInstructions(toolHome, content string) error {
	return writeManagedInstructions(copilotInstructionsHostPath(toolHome), content)
}
