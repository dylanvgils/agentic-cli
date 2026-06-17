// Package buildinfo classifies the agentic CLI's own version string.
package buildinfo

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
