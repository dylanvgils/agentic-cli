package sidecar

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
)

// fakeDocker implements Docker; a nil field succeeds with a zero value.
type fakeDocker struct {
	inspectImage    func(string) (*docker.ImageInfo, error)
	serverArch      func() (string, error)
	buildProxyImage func(string, string, string, tools.BuildOptions) error
	buildDindImage  func(string, tools.BuildOptions) error
	pruneImages     func() (string, error)
}

func (f *fakeDocker) InspectImage(name string) (*docker.ImageInfo, error) {
	if f.inspectImage == nil {
		return nil, nil
	}
	return f.inspectImage(name)
}

// ServerArch reports an amd64 daemon unless stubbed.
func (f *fakeDocker) ServerArch() (string, error) {
	if f.serverArch == nil {
		return "amd64", nil
	}
	return f.serverArch()
}

func (f *fakeDocker) BuildProxyImage(image, version, contextDir string, opts tools.BuildOptions) error {
	if f.buildProxyImage == nil {
		return nil
	}
	return f.buildProxyImage(image, version, contextDir, opts)
}

func (f *fakeDocker) BuildDindImage(image string, opts tools.BuildOptions) error {
	if f.buildDindImage == nil {
		return nil
	}
	return f.buildDindImage(image, opts)
}

func (f *fakeDocker) PruneImages() (string, error) {
	if f.pruneImages == nil {
		return "", nil
	}
	return f.pruneImages()
}

// fakeReleases implements Releases; a nil downloadProxy writes a placeholder binary to destPath.
type fakeReleases struct {
	downloadProxy func(version, arch, destPath string) error
}

func (f *fakeReleases) DownloadProxy(version, arch, destPath string) error {
	if f.downloadProxy == nil {
		return os.WriteFile(destPath, []byte("proxy binary"), 0o755)
	}
	return f.downloadProxy(version, arch, destPath)
}

// stubErrLog redirects logging.Err to a buffer for the duration of the test and returns it.
func stubErrLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	orig := logging.Err
	logging.Err = logging.New(&buf)
	t.Cleanup(func() { logging.Err = orig })

	return &buf
}

// recordBuilds returns a fakeDocker reporting every image missing, and a getter for the images built.
func recordBuilds() (*fakeDocker, func() []string) {
	var built []string
	fake := &fakeDocker{
		buildProxyImage: func(image, _, _ string, _ tools.BuildOptions) error {
			built = append(built, image)
			return nil
		},
		buildDindImage: func(image string, _ tools.BuildOptions) error {
			built = append(built, image)
			return nil
		},
	}
	return fake, func() []string { return built }
}

// inspectReturns returns an InspectImage func that always yields info and err.
func inspectReturns(info *docker.ImageInfo, err error) func(string) (*docker.ImageInfo, error) {
	return func(string) (*docker.ImageInfo, error) { return info, err }
}

// formatTestLabelTime formats t like agentic's image timestamp labels.
func formatTestLabelTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}
