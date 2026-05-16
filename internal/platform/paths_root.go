package platform

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// FindRepoRoot resolves the repository root by following the agentic symlink on PATH.
// Returns an error if agentic is not on PATH or its real path cannot be resolved.
func FindRepoRoot() (string, error) {
	path := lookupBinary("agentic")
	if path == "" {
		return "", fmt.Errorf("agentic not found on PATH")
	}

	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve agentic path: %w", err)
	}

	return filepath.Dir(filepath.Dir(real)), nil
}

func lookupBinary(name string) string {
	path, _ := exec.LookPath(name)
	return path
}
