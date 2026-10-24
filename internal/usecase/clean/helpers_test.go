package clean

import (
	"bytes"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
)

// fakeDocker implements Docker; a nil field succeeds with a zero value.
type fakeDocker struct {
	listAllImages       func(...docker.ImageFilter) ([]*docker.ImageInfo, error)
	listRunning         func() ([]*docker.ContainerInfo, error)
	cleanImage          func(string) error
	cleanBaseImages     func() error
	sweepProxyResources func() ([]string, error)
	sweepDindResources  func(string) ([]string, error)
	sweepRunNetworks    func() ([]string, error)
	removeNetwork       func() error
	pruneImages         func() (string, error)
	pruneBuildCache     func() (string, error)
}

func (f *fakeDocker) ListAllImages(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
	if f.listAllImages == nil {
		return nil, nil
	}
	return f.listAllImages(filters...)
}

func (f *fakeDocker) ListRunningContainers() ([]*docker.ContainerInfo, error) {
	if f.listRunning == nil {
		return nil, nil
	}
	return f.listRunning()
}

func (f *fakeDocker) CleanImage(image string) error {
	if f.cleanImage == nil {
		return nil
	}
	return f.cleanImage(image)
}

func (f *fakeDocker) CleanBaseImages() error {
	if f.cleanBaseImages == nil {
		return nil
	}
	return f.cleanBaseImages()
}

func (f *fakeDocker) SweepProxyResources() ([]string, error) {
	if f.sweepProxyResources == nil {
		return nil, nil
	}
	return f.sweepProxyResources()
}

func (f *fakeDocker) SweepDindResources(toolHome string) ([]string, error) {
	if f.sweepDindResources == nil {
		return nil, nil
	}
	return f.sweepDindResources(toolHome)
}

func (f *fakeDocker) SweepRunNetworks() ([]string, error) {
	if f.sweepRunNetworks == nil {
		return nil, nil
	}
	return f.sweepRunNetworks()
}

func (f *fakeDocker) RemoveNetwork() error {
	if f.removeNetwork == nil {
		return nil
	}
	return f.removeNetwork()
}

func (f *fakeDocker) PruneImages() (string, error) {
	if f.pruneImages == nil {
		return "", nil
	}
	return f.pruneImages()
}

func (f *fakeDocker) PruneBuildCache() (string, error) {
	if f.pruneBuildCache == nil {
		return "", nil
	}
	return f.pruneBuildCache()
}

// fakeRuntime implements ImageCleaner; a nil field succeeds.
type fakeRuntime struct {
	cleanImage func(string) error
}

func (f *fakeRuntime) CleanImage(image string) error {
	if f.cleanImage == nil {
		return nil
	}
	return f.cleanImage(image)
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
