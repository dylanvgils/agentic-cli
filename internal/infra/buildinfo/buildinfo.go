// Package buildinfo classifies the agentic CLI's own version string.
package buildinfo

import (
	"os"
	"path/filepath"
	"strings"
)

// Version, Commit, BuildDate, and InstallMethod are injected at build time via -ldflags (see Makefile and .goreleaser.yml).
var (
	Version       = "dev"
	Commit        = ""
	BuildDate     = ""
	InstallMethod = ""
)

// IsDev reports whether version denotes an unreleased dev build.
func IsDev(version string) bool {
	return version == "" || version == "dev"
}

// IsDevBuild reports whether the running agentic binary itself is a dev build.
func IsDevBuild() bool {
	return IsDev(Version)
}

// DevSourceDir returns the local module root for modulePath, for use as a dev build's Docker build context - "" for released builds or when no matching go.mod is found.
func DevSourceDir(modulePath string) string {
	if !IsDev(Version) {
		return ""
	}
	return findModuleRoot(modulePath)
}

// findModuleRoot walks up from the working directory for a go.mod matching modulePath, returning its directory or "" if not found.
func findModuleRoot(modulePath string) string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil && moduleMatches(data, modulePath) {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// moduleMatches reports whether a go.mod declares the given module path.
func moduleMatches(gomod []byte, modulePath string) bool {
	for line := range strings.SplitSeq(string(gomod), "\n") {
		if after, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(after) == modulePath
		}
	}
	return false
}
