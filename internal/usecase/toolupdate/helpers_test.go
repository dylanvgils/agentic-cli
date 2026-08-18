package toolupdate

import (
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
)

func stubLatestToolVersion(t *testing.T, fn func(tool, installedLabel string) (string, bool, bool)) {
	t.Helper()
	orig := LatestToolVersion
	LatestToolVersion = fn
	t.Cleanup(func() { LatestToolVersion = orig })
}

func stubInspectImage(t *testing.T, info *docker.ImageInfo, err error) {
	t.Helper()
	orig := InspectImage
	InspectImage = func(string) (*docker.ImageInfo, error) { return info, err }
	t.Cleanup(func() { InspectImage = orig })
}

func stubIsTerminal(t *testing.T, terminal bool) {
	t.Helper()
	orig := IsTerminal
	IsTerminal = func() bool { return terminal }
	t.Cleanup(func() { IsTerminal = orig })
}

func stubStdin(t *testing.T, input string) {
	t.Helper()
	orig := Stdin
	Stdin = strings.NewReader(input)
	t.Cleanup(func() { Stdin = orig })
}
