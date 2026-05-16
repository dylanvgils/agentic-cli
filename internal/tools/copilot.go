package tools

import (
	"os"
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/mount"
)

func copilotMounts() []string {
	return []string{
		mount.VolumeMount("$PWD", "/workspace"),
		mount.VolumeMount("$TOOL_HOME/copilot", "$CONTAINER_HOME/.copilot"),
	}
}

func setupCopilot(toolHome string) error {
	return os.MkdirAll(filepath.Join(toolHome, "copilot"), 0o750)
}
