package clean

import (
	"bytes"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
)

func stubListAllImages(t *testing.T, fn func(...docker.ImageFilter) ([]*docker.ImageInfo, error)) {
	t.Helper()
	orig := ListAllImages
	ListAllImages = fn
	t.Cleanup(func() { ListAllImages = orig })
}

func stubCleanImage(t *testing.T, fn func(string) error) {
	t.Helper()
	orig := CleanImage
	CleanImage = fn
	t.Cleanup(func() { CleanImage = orig })
}

func stubCleanBaseImages(t *testing.T, fn func() error) {
	t.Helper()
	orig := CleanBaseImages
	CleanBaseImages = fn
	t.Cleanup(func() { CleanBaseImages = orig })
}

func stubSweepProxyResources(t *testing.T, fn func() error) {
	t.Helper()
	orig := SweepProxyResources
	SweepProxyResources = fn
	t.Cleanup(func() { SweepProxyResources = orig })
}

func stubSweepDindResources(t *testing.T, fn func(string) error) {
	t.Helper()
	orig := SweepDindResources
	SweepDindResources = fn
	t.Cleanup(func() { SweepDindResources = orig })
}

func stubRemoveNetwork(t *testing.T, fn func() error) {
	t.Helper()
	orig := RemoveNetwork
	RemoveNetwork = fn
	t.Cleanup(func() { RemoveNetwork = orig })
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
