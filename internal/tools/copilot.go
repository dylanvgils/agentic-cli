package tools

import (
	"os"
	"path/filepath"

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

func setupCopilot(toolHome string) error {
	return os.MkdirAll(filepath.Join(toolHome, "copilot"), 0o750)
}
