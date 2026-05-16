//go:build windows

package platform

import (
	"os"
	"path/filepath"
)

func toolHomeDefault() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ".agentic"
	}
	return filepath.Join(appData, "agentic")
}

// GetUID returns a placeholder on Windows (no UID concept).
func GetUID() string {
	return "1000"
}

// GetGID returns a placeholder on Windows (no GID concept).
func GetGID() string {
	return "1000"
}

// UserGroup returns "UID:GID" placeholder for the --user docker flag on Windows.
func UserGroup() string {
	return "1000:1000"
}
