package tools

import (
	"os"
	"path/filepath"

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
		mount.VolumeMount("$TOOL_HOME/opencode/data", "$CONTAINER_HOME/.local/share/opencode"),
		mount.VolumeMount("$TOOL_HOME/opencode/cache", "$CONTAINER_HOME/.cache/opencode"),
		mount.VolumeMount("$TOOL_HOME/opencode/state", "$CONTAINER_HOME/.local/state/opencode"),
	}
}

func setupOpencode(toolHome string) error {
	for _, sub := range []string{"data", "cache", "state"} {
		if err := os.MkdirAll(filepath.Join(toolHome, "opencode", sub), 0o750); err != nil {
			return err
		}
	}
	return nil
}
