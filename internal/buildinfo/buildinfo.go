// Package buildinfo classifies the agentic CLI's own version string.
package buildinfo

import (
	"os"
	"path/filepath"
	"strings"
)

// Version, Commit, BuildDate, and InstallMethod are injected at build time via
// -ldflags (see Makefile and .goreleaser.yml). Version defaults to "dev" for
// local, unreleased builds.
var (
	Version       = "dev"
	Commit        = ""
	BuildDate     = ""
	InstallMethod = ""
)

// IsDev reports whether version denotes an unreleased dev build. Dev builds
// compile the proxy from the local source tree; released builds install the
// published module via `go install`.
func IsDev(version string) bool {
	return version == "" || version == "dev"
}

// IsDevBuild reports whether the running agentic binary itself is a dev build.
func IsDevBuild() bool {
	return IsDev(Version)
}

// DevSourceDir returns the local module root for modulePath, for use as a dev
// build's Docker build context. It returns "" for released builds (which
// install the published module instead) and when no matching go.mod is found
// by walking up from the working directory.
func DevSourceDir(modulePath string) string {
	if !IsDev(Version) {
		return ""
	}
	return findModuleRoot(modulePath)
}

// findModuleRoot walks up from the working directory looking for the go.mod of
// the given module, returning its directory or "" if not found. It verifies the
// module path so an unrelated project's go.mod is never used as source.
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
