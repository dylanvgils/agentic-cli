package tools

import (
	"os"
	"path/filepath"

	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
	"github.com/dylanvgils/agentic-cli/internal/mount"
)

func copilotTmpfsMounts() []string {
	return []string{
		mount.TmpfsMount("/tmp", mount.TmpfsOptions{Exec: true, Size: "1g"}),
		mount.TmpfsMount("$CONTAINER_HOME/.cache", mount.TmpfsOptions{Exec: true, Size: "1g"}),
	}
}

func copilotMounts() []string {
	return []string{
		mount.VolumeMount("$PWD", "/workspace"),
		mount.VolumeMount("$TOOL_HOME/copilot", "$CONTAINER_HOME/.copilot"),
	}
}

func copilotStage(prevStage string) df.Stage {
	return df.NewStage(df.From{Image: prevStage, As: "tool"}).
		Add(df.Shell{Cmd: []string{"/bin/bash", "-o", "pipefail", "-c"}}).
		Add(df.Arg{Key: "HOST_UID", Default: "1000"}).
		Add(df.Arg{Key: "HOST_GID", Default: "1000"}).
		Add(df.Label{Key: "project", Value: "agentic-cli"}).
		Add(df.Run{Blocks: []df.Block{
			{
				Comment: "Remove conflicting user at HOST_UID",
				Lines: []string{
					`existing=$(getent passwd ${HOST_UID} | cut -d: -f1);`,
					`if [ -n "$existing" ] && [ "$existing" != "copilot" ]; then`,
					`userdel -r "$existing" 2>/dev/null || true;`,
					`fi`,
				},
			},
			{Comment: "Create container user", Lines: []string{`groupadd -g ${HOST_GID} --non-unique copilot`}},
			{Lines: []string{`useradd -l -u ${HOST_UID} -g ${HOST_GID} -m -s /bin/bash --non-unique copilot`}},
		}}).
		Add(df.Run{Command: "curl -fsSL https://gh.io/copilot-install | bash"}).
		Add(df.Heredoc{
			Dest:  "/usr/local/bin/" + versionScript("copilot"),
			Lines: []string{"#!/bin/sh", "copilot --version"},
		}).
		Add(df.Heredoc{
			Dest: "/usr/local/bin/entrypoint.sh",
			Lines: []string{
				"#!/usr/bin/env bash",
				"set -euo pipefail",
				"",
				"# Set GITHUB_TOKEN if mounted in container",
				"if [[ -f /run/secrets/copilot_token ]]; then",
				`  export GITHUB_TOKEN="$(cat /run/secrets/copilot_token)"`,
				"fi",
				"",
				`exec copilot "$@"`,
			},
		}).
		Add(df.Run{Command: "mkdir -p /home/copilot/.copilot"}).
		Add(df.User{Name: "copilot"}).
		Add(df.Env{Key: "TOOL_HOME", Value: "/home/copilot"}).
		Add(df.Workdir{Path: "/workspace"}).
		Add(df.Entrypoint{Cmd: []string{"/usr/local/bin/entrypoint.sh"}}).
		Build()
}

func setupCopilot(toolHome string) error {
	return os.MkdirAll(filepath.Join(toolHome, "copilot"), 0o750)
}
