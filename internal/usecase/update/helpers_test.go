package update

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/stretchr/testify/require"
)

// fakeDocker implements Docker; a nil field succeeds with a zero value.
type fakeDocker struct {
	listAllImages func(...docker.ImageFilter) ([]*docker.ImageInfo, error)
	inspectImage  func(string) (*docker.ImageInfo, error)
	buildTool     func(tool, image string, opts tools.BuildOptions) error
	restampImage  func(image string, info docker.ImageInfo)
	pruneImages   func() (string, error)
}

func (f *fakeDocker) ListAllImages(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
	if f.listAllImages == nil {
		return nil, nil
	}
	return f.listAllImages(filters...)
}

func (f *fakeDocker) InspectImage(name string) (*docker.ImageInfo, error) {
	if f.inspectImage == nil {
		return nil, nil
	}
	return f.inspectImage(name)
}

func (f *fakeDocker) BuildTool(tool, image string, opts tools.BuildOptions) error {
	if f.buildTool == nil {
		return nil
	}
	return f.buildTool(tool, image, opts)
}

func (f *fakeDocker) RestampImage(image string, info docker.ImageInfo) {
	if f.restampImage != nil {
		f.restampImage(image, info)
	}
}

func (f *fakeDocker) PruneImages() (string, error) {
	if f.pruneImages == nil {
		return "", nil
	}
	return f.pruneImages()
}

// inspectReturns returns an InspectImage func that always yields info and err.
func inspectReturns(info *docker.ImageInfo, err error) func(string) (*docker.ImageInfo, error) {
	return func(string) (*docker.ImageInfo, error) { return info, err }
}

// fakeUpstream implements Upstream, returning latest, newer and ok and counting the calls.
type fakeUpstream struct {
	latest string
	newer  bool
	ok     bool
	calls  int
}

func (f *fakeUpstream) LatestToolVersion(string, string) (string, bool, bool) {
	f.calls++
	return f.latest, f.newer, f.ok
}

// storeCustomInstalls saves installs to the custom installs store under home and returns their hash.
func storeCustomInstalls(t *testing.T, home string, installs ...config.RCCustomInstall) string {
	t.Helper()
	require.NoError(t, config.SaveCustomInstalls(home, installs))
	return config.CustomInstallsHash(installs)
}

// builds records the BuildTool calls a fakeDocker gets.
type builds struct {
	images []string
	opts   []tools.BuildOptions
}

// newTestService returns a Service over d and u with a temp data dir.
func newTestService(t *testing.T, d Docker, u Upstream) *Service {
	t.Helper()
	return New(d, u, t.TempDir())
}

// stubBuilds makes d's BuildTool record each build and return err.
func stubBuilds(d *fakeDocker, err error) *builds {
	b := &builds{}
	d.buildTool = func(_, image string, opts tools.BuildOptions) error {
		b.images = append(b.images, image)
		b.opts = append(b.opts, opts)
		return err
	}
	return b
}
