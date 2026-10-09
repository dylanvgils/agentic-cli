package build

import (
	"bytes"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// fakeDocker implements Docker; a nil field succeeds with a zero value.
type fakeDocker struct {
	buildTool     func(tool, image string, opts tools.BuildOptions) error
	pruneDangling func() error
}

func (f *fakeDocker) BuildTool(tool, image string, opts tools.BuildOptions) error {
	if f.buildTool == nil {
		return nil
	}
	return f.buildTool(tool, image, opts)
}

func (f *fakeDocker) PruneDangling() error {
	if f.pruneDangling == nil {
		return nil
	}
	return f.pruneDangling()
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
