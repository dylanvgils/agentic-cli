package toolupdate

import (
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
)

// fakeDocker implements Docker; a nil field succeeds with a zero value.
type fakeDocker struct {
	inspectImage func(string) (*docker.ImageInfo, error)
}

func (f *fakeDocker) InspectImage(image string) (*docker.ImageInfo, error) {
	if f.inspectImage == nil {
		return nil, nil
	}
	return f.inspectImage(image)
}

func stubLatestToolVersion(t *testing.T, fn func(tool, installedLabel string) (string, bool, bool)) {
	t.Helper()
	orig := LatestToolVersion
	LatestToolVersion = fn
	t.Cleanup(func() { LatestToolVersion = orig })
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

// inspectReturns returns an InspectImage func that always yields info and err.
func inspectReturns(info *docker.ImageInfo, err error) func(string) (*docker.ImageInfo, error) {
	return func(string) (*docker.ImageInfo, error) { return info, err }
}
