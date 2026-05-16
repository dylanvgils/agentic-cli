package platform

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// repoRoot may be set at build time via -ldflags (injected by make install).
var repoRoot string

// FindRepoRoot resolves the repository root. When installed via make install the
// path is embedded at build time; otherwise it falls back to following the agentic
// symlink on PATH. Returns an error if the root cannot be determined.
func FindRepoRoot() (string, error) {
	if repoRoot != "" {
		return repoRoot, nil
	}

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
