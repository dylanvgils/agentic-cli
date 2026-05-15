package tools

import (
	"os"
	"path/filepath"
)

func copilotMounts(home string) []string {
	mounts := []string{
		"$PWD:/workspace",
		"$TOOL_HOME/copilot:$CONTAINER_HOME/.copilot",
	}

	tokenPath := filepath.Join(home, ".secrets", "copilot_token")
	if _, err := os.Stat(tokenPath); err == nil {
		mounts = append(mounts, tokenPath+":/run/secrets/copilot_token:ro")
	}

	return mounts
}

func setupCopilot(toolHome string) error {
	return os.MkdirAll(filepath.Join(toolHome, "copilot"), 0o750)
}
