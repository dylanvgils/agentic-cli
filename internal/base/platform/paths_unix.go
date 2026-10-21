//go:build !windows

package platform

import (
	"os"
	"path/filepath"
)

func toolHomeDefault() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".agentic"
	}
	return filepath.Join(home, ".agentic")
}
