package toolupdate

import "github.com/dylanvgils/agentic-cli/internal/docker"

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

// fakeUpstream implements Upstream with the func itself.
type fakeUpstream func(tool, installedLabel string) (string, bool, bool)

func (f fakeUpstream) LatestToolVersion(tool, installedLabel string) (string, bool, bool) {
	return f(tool, installedLabel)
}

// inspectReturns returns an InspectImage func that always yields info and err.
func inspectReturns(info *docker.ImageInfo, err error) func(string) (*docker.ImageInfo, error) {
	return func(string) (*docker.ImageInfo, error) { return info, err }
}
