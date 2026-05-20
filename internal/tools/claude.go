package tools

import (
	"os"
	"path/filepath"

	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
	"github.com/dylanvgils/agentic-cli/internal/mount"
)

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

func claudeStage(prevStage string) df.Stage {
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
						`if [ -n "$existing" ] && [ "$existing" != "claude" ]; then`,
						`userdel -r "$existing" 2>/dev/null || true;`,
						`fi`,
					},
				},
				{Comment: "Create container user", Lines: []string{`groupadd -g ${HOST_GID} --non-unique claude`}},
				{Lines: []string{`useradd -l -u ${HOST_UID} -g ${HOST_GID} -m -s /bin/bash --non-unique claude`}},
			}},
			df.Heredoc{
				Dest:  "/usr/local/bin/entrypoint.sh",
				Lines: []string{"#!/usr/bin/env bash", "set -euo pipefail", "exec claude \"$@\""},
			},
			df.User{Name: "claude"},
			df.Env{Key: "PATH", Value: "/home/claude/.local/bin:${PATH}"},
			df.Run{Blocks: []df.Block{
				{Lines: []string{"curl -fsSL https://claude.ai/install.sh | bash"}},
				{Lines: []string{`mkdir -p "/home/claude/.claude"`}},
			}},
			df.Env{Key: "TOOL_HOME", Value: "/home/claude"},
			df.Workdir{Path: "/workspace"},
			df.Entrypoint{Cmd: []string{"/usr/local/bin/entrypoint.sh"}},
		},
	}
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
