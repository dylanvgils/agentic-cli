package update

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/require"
)

// fakeDocker implements Docker; a nil field succeeds with a zero value.
type fakeDocker struct {
	listAllImages func(...docker.ImageFilter) ([]*docker.ImageInfo, error)
	inspectImage  func(string) (*docker.ImageInfo, error)
	buildTool     func(tool, image string, opts tools.BuildOptions) error
	restampImage  func(image string, info docker.ImageInfo)
	pruneDangling func() error
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

func (f *fakeDocker) PruneDangling() error {
	if f.pruneDangling == nil {
		return nil
	}
	return f.pruneDangling()
}

// inspectReturns returns an InspectImage func that always yields info and err.
func inspectReturns(info *docker.ImageInfo, err error) func(string) (*docker.ImageInfo, error) {
	return func(string) (*docker.ImageInfo, error) { return info, err }
}

// inspectSequence returns an InspectImage func that yields results in order, repeating the last one.
func inspectSequence(results ...*docker.ImageInfo) func(string) (*docker.ImageInfo, error) {
	call := 0
	return func(string) (*docker.ImageInfo, error) {
		idx := min(call, len(results)-1)
		call++
		return results[idx], nil
	}
}

// stubLatestToolVersion makes LatestToolVersion return latest, newer, ok and returns a getter for how often it was called.
func stubLatestToolVersion(t *testing.T, latest string, newer, ok bool) func() int {
	t.Helper()
	calls := 0
	orig := LatestToolVersion
	LatestToolVersion = func(string, string) (string, bool, bool) {
		calls++
		return latest, newer, ok
	}
	t.Cleanup(func() { LatestToolVersion = orig })

	return func() int { return calls }
}

// captureStdout replaces os.Stdout with a pipe and returns what was written (e.g. DryRun's Dockerfile output); for logging.Step/Detail output, use captureLog.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stdout
	os.Stdout = w
	fn()
	w.Close() //nolint:errcheck
	os.Stdout = orig
	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck
	return buf.String()
}

// captureLog swaps logging.Log for the duration of fn and returns what was written.
func captureLog(t *testing.T, fn func()) string {
	t.Helper()

	var buf bytes.Buffer
	orig := logging.Log
	logging.Log = logging.New(&buf)
	t.Cleanup(func() { logging.Log = orig })

	fn()

	return buf.String()
}
