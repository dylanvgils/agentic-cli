package clean

import (
	"bytes"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
)

// fakeDocker implements Docker; a nil field succeeds with a zero value.
type fakeDocker struct {
	listAllImages       func(...docker.ImageFilter) ([]*docker.ImageInfo, error)
	cleanImage          func(string) error
	cleanBaseImages     func() error
	sweepProxyResources func() error
	sweepDindResources  func(string) error
	removeNetwork       func() error
}

func (f *fakeDocker) ListAllImages(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
	if f.listAllImages == nil {
		return nil, nil
	}
	return f.listAllImages(filters...)
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

func (f *fakeDocker) SweepProxyResources() error {
	if f.sweepProxyResources == nil {
		return nil
	}
	return f.sweepProxyResources()
}

func (f *fakeDocker) SweepDindResources(toolHome string) error {
	if f.sweepDindResources == nil {
		return nil
	}
	return f.sweepDindResources(toolHome)
}

func (f *fakeDocker) RemoveNetwork() error {
	if f.removeNetwork == nil {
		return nil
	}
	return f.removeNetwork()
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
