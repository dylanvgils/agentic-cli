package tools

import (
	"os"
	"path/filepath"

	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
	"github.com/dylanvgils/agentic-cli/internal/mount"
)

func opencodeTmpfsMounts() []string {
	return []string{
		mount.TmpfsMount("/tmp", mount.TmpfsOptions{Exec: true, Size: "1g"}),
	}
}

func opencodeMounts() []string {
	return []string{
		mount.VolumeMount("$PWD", "/workspace"),
		mount.VolumeMount("$TOOL_HOME/opencode/data", "$CONTAINER_HOME/.opencode"),
		mount.VolumeMount("$TOOL_HOME/opencode/share", "$CONTAINER_HOME/.local/share/opencode"),
		mount.VolumeMount("$TOOL_HOME/opencode/state", "$CONTAINER_HOME/.local/state/opencode"),
		mount.VolumeMount("$TOOL_HOME/opencode/cache", "$CONTAINER_HOME/.cache/opencode"),
		mount.VolumeMount("$TOOL_HOME/opencode/config", "$CONTAINER_HOME/.config/opencode"),
	}
}

func opencodeStage(prevStage string) df.Stage {
	return df.Stage{
		From: df.From{Image: prevStage, As: "tool"},
		Instructions: []df.Instruction{
			df.Shell{Form: []string{"/bin/bash", "-o", "pipefail", "-c"}},
			df.Arg{Key: "HOST_UID", Default: "1000"},
			df.Arg{Key: "HOST_GID", Default: "1000"},
			df.Label{Key: "project", Value: "agentic-cli"},
			df.Run{Blocks: []df.Block{
				{
					Comment: "Remove conflicting user at HOST_UID",
					Lines: []string{
						`existing=$(getent passwd ${HOST_UID} | cut -d: -f1);`,
						`if [ -n "$existing" ] && [ "$existing" != "opencode" ]; then`,
						`userdel -r "$existing" 2>/dev/null || true;`,
						`fi`,
					},
				},
				{Comment: "Create container user", Lines: []string{`groupadd -g ${HOST_GID} --non-unique opencode`}},
				{Lines: []string{`useradd -l -u ${HOST_UID} -g ${HOST_GID} -m -s /bin/bash --non-unique opencode`}},
			}},
			df.Heredoc{
				Dest:  "/usr/local/bin/entrypoint.sh",
				Lines: []string{"#!/usr/bin/env bash", "set -euo pipefail", `exec opencode "$@"`},
			},
			df.Run{Blocks: []df.Block{
				{Lines: []string{"curl -fsSL https://opencode.ai/install | bash -s -- --no-modify-path"}},
				{Lines: []string{"mv /root/.opencode/bin/opencode /usr/local/bin/opencode"}},
				{Lines: []string{"rm -rf /root/.opencode"}},
			}},
			df.User{Name: "opencode"},
			df.Env{Key: "TOOL_HOME", Value: "/home/opencode"},
			df.Workdir{Path: "/workspace"},
			df.Entrypoint{Cmd: []string{"/usr/local/bin/entrypoint.sh"}},
		},
	}
}

func setupOpencode(toolHome string) error {
	for _, sub := range []string{"data", "share", "state", "cache", "config"} {
		if err := os.MkdirAll(filepath.Join(toolHome, "opencode", sub), 0o750); err != nil {
			return err
		}
	}
	return nil
}
