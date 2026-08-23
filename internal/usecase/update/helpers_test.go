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

func stubListAllImages(t *testing.T, fn func(...docker.ImageFilter) ([]*docker.ImageInfo, error)) {
	t.Helper()
	orig := ListAllImages
	ListAllImages = fn
	t.Cleanup(func() { ListAllImages = orig })
}

func stubInspectImage(t *testing.T, info *docker.ImageInfo, err error) {
	t.Helper()
	orig := InspectImage
	InspectImage = func(string) (*docker.ImageInfo, error) { return info, err }
	t.Cleanup(func() { InspectImage = orig })
}

// stubInspectImageSequence returns results in order by call, repeating the last entry for any further calls.
func stubInspectImageSequence(t *testing.T, results ...*docker.ImageInfo) {
	t.Helper()
	orig := InspectImage
	call := 0
	InspectImage = func(string) (*docker.ImageInfo, error) {
		idx := call
		if idx >= len(results) {
			idx = len(results) - 1
		}
		call++
		return results[idx], nil
	}
	t.Cleanup(func() { InspectImage = orig })
}

func stubUpdateTool(t *testing.T, fn func(tool, image string, opts tools.BuildOptions) error) {
	t.Helper()
	orig := UpdateTool
	UpdateTool = fn
	t.Cleanup(func() { UpdateTool = orig })
}

func stubLatestToolVersion(t *testing.T, latest string, newer, ok bool) {
	t.Helper()
	orig := LatestToolVersion
	LatestToolVersion = func(string, string) (string, bool, bool) { return latest, newer, ok }
	t.Cleanup(func() { LatestToolVersion = orig })
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
