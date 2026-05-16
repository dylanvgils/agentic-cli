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

