package build

import (
	"bytes"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
)

// fakeDocker implements Docker; a nil field succeeds with a zero value.
type fakeDocker struct {
	buildTool   func(tool, image string, opts tools.BuildOptions) error
	pruneImages func() (string, error)
}

func (f *fakeDocker) BuildTool(tool, image string, opts tools.BuildOptions) error {
	if f.buildTool == nil {
		return nil
	}
	return f.buildTool(tool, image, opts)
}

func (f *fakeDocker) PruneImages() (string, error) {
	if f.pruneImages == nil {
		return "", nil
	}
	return f.pruneImages()
}

// stubOut redirects logging.Out to a buffer for the duration of the test and returns it.
func stubOut(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	orig := logging.Out
	logging.Out = &buf
	t.Cleanup(func() { logging.Out = orig })

	return &buf
}

// captureErrLog swaps logging.Err, where progress goes, for the duration of fn and returns what was written.
func captureErrLog(t *testing.T, fn func()) string {
	t.Helper()

	buf := stubErrLog(t)
	fn()

	return buf.String()
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
