package update

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
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

func stubUpdateTool(t *testing.T, fn func(tool, image string, opts tools.BuildOptions) error) {
	t.Helper()
	orig := UpdateTool
	UpdateTool = fn
	t.Cleanup(func() { UpdateTool = orig })
}

// captureStdout replaces os.Stdout with a pipe and returns what was written.
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
