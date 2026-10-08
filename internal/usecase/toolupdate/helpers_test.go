package toolupdate

import (
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

// inspectReturns returns an InspectImage func that always yields info and err.
func inspectReturns(info *docker.ImageInfo, err error) func(string) (*docker.ImageInfo, error) {
	return func(string) (*docker.ImageInfo, error) { return info, err }
}
