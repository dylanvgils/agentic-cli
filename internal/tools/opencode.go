package tools

import (
	"os"
	"path"
	"path/filepath"

	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
	"github.com/dylanvgils/agentic-cli/internal/mount"
)

// opencodeReleaseRepo is the GitHub repo the install script (opencode.ai/install) pulls releases from.
const opencodeReleaseRepo = "anomalyco/opencode"

// opencodeInstructionsContainerPath is the container-side mount target for a per-run instructions snapshot.
const opencodeInstructionsContainerPath = "$CONTAINER_HOME/.config/opencode/AGENTS.md"

// opencodeAllowedHosts is the baseline allowlist; OpenCode is multi-provider, so users add their model-provider hosts via allowed_hosts.
var opencodeAllowedHosts = []string{
	"opencode.ai", // OpenCode auth and update checks
}

func opencodeTmpfsMounts() []string {
	return []string{
		mount.TmpfsMount("/tmp", mount.TmpfsOptions{Exec: true, Size: "1g"}),
	}
}

func opencodeMounts() []string {
	return []string{
		mount.VolumeMount("$PWD", mount.WorkspaceContainerPath),
		mount.VolumeMount(path.Join("$TOOL_HOME", ToolsDirName, "opencode", "data"), "$CONTAINER_HOME/.opencode"),
		mount.VolumeMount(path.Join("$TOOL_HOME", ToolsDirName, "opencode", "share"), "$CONTAINER_HOME/.local/share/opencode"),
		mount.VolumeMount(path.Join("$TOOL_HOME", ToolsDirName, "opencode", "state"), "$CONTAINER_HOME/.local/state/opencode"),
		mount.VolumeMount(path.Join("$TOOL_HOME", ToolsDirName, "opencode", "cache"), "$CONTAINER_HOME/.cache/opencode"),
		mount.VolumeMount(path.Join("$TOOL_HOME", ToolsDirName, "opencode", "config"), "$CONTAINER_HOME/.config/opencode"),
	}
}

func opencodeStage(prevStage string) df.Stage {
	return df.NewStage(df.From{Image: prevStage, As: "tool"}).
		Add(df.Shell{Cmd: []string{"/bin/bash", "-o", "pipefail", "-c"}}).
		Add(createContainerUser("opencode")...).
		Add(df.Heredoc{
			Dest:  "/usr/local/bin/entrypoint.sh",
			Lines: []string{"#!/usr/bin/env bash", "set -euo pipefail", `exec opencode "$@"`},
		}).
		Add(df.Run{Blocks: []df.Block{
			{Lines: []string{"curl -fsSL https://opencode.ai/install | bash -s -- --no-modify-path"}},
			{Lines: []string{"mv /root/.opencode/bin/opencode /usr/local/bin/opencode"}},
			{Lines: []string{"rm -rf /root/.opencode"}},
		}}).
		Add(df.Heredoc{
			Dest:  "/usr/local/bin/" + versionScript("opencode"),
			Lines: []string{"#!/bin/sh", "opencode --version"},
		}).
		Add(df.User{Name: "opencode"}).
		Add(df.Env{Key: "TOOL_HOME", Value: "/home/opencode"}).
		Add(df.Env{Key: "OPENCODE_DISABLE_AUTOUPDATE", Value: "true"}).
		Add(df.Workdir{Path: mount.WorkspaceContainerPath}).
		Add(df.Entrypoint{Cmd: []string{"/usr/local/bin/entrypoint.sh"}}).
		Build()
}

// opencodeLatestVersion fetches the latest OpenCode version available upstream.
func opencodeLatestVersion() (string, error) {
	return latestGithubTag(opencodeReleaseRepo)
}

func setupOpencode(toolHome string) error {
	for _, sub := range []string{"data", "share", "state", "cache", "config"} {
		if err := os.MkdirAll(filepath.Join(toolHome, ToolsDirName, "opencode", sub), 0o750); err != nil {
			return err
		}
	}
	return nil
}

// opencodeInstructionsHostPath is OpenCode's global AGENTS.md (~/.config/opencode/AGENTS.md).
func opencodeInstructionsHostPath(toolHome string) string {
	return filepath.Join(toolHome, ToolsDirName, "opencode", "config", "AGENTS.md")
}

// writeOpencodeInstructions writes content to OpenCode's global AGENTS.md.
func writeOpencodeInstructions(toolHome, content string) error {
	return writeManagedInstructions(opencodeInstructionsHostPath(toolHome), content)
}
