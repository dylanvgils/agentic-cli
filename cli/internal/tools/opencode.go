package tools

import (
	"os"
	"path/filepath"
)

func opencodeMounts(_ string) []string {
	return []string{
		"$PWD:/workspace",
		"$TOOL_HOME/opencode/data:$CONTAINER_HOME/.local/share/opencode",
		"$TOOL_HOME/opencode/cache:$CONTAINER_HOME/.cache/opencode",
		"$TOOL_HOME/opencode/state:$CONTAINER_HOME/.local/state/opencode",
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
