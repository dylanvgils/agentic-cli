package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureStdout replaces os.Stdout with a pipe and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck
	return buf.String()
}

// stubInspectImage replaces inspectImage with a func that returns the given info/err for any image.
func stubInspectImage(t *testing.T, info *docker.ImageInfo, err error) func() {
	t.Helper()
	orig := inspectImage
	inspectImage = func(_ string) (*docker.ImageInfo, error) { return info, err }
	return func() { inspectImage = orig }
}

var builtInfo = &docker.ImageInfo{
	Image:   "agentic-claude",
	ID:      "a1b2c3d4e5f6",
	Version: "1.2.3",
	Base:    "node:24",
	Built:   "2026-05-01",
	SizeMB:  512,
}

func TestRunInspect_allTools_whenNoArgs(t *testing.T) {
	// Arrange
	restore := stubInspectImage(t, builtInfo, nil)
	defer restore()

	// Act
	out := captureStdout(t, func() {
		err := runInspect(inspectCmd, []string{})
		require.NoError(t, err)
	})

	// Assert - all three tools should appear
	assert.Contains(t, out, "=> claude")
	assert.Contains(t, out, "=> copilot")
	assert.Contains(t, out, "=> opencode")
}

func TestRunInspect_singleTool_whenArgGiven(t *testing.T) {
	// Arrange
	restore := stubInspectImage(t, builtInfo, nil)
	defer restore()

	// Act
	out := captureStdout(t, func() {
		err := runInspect(inspectCmd, []string{"claude"})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "=> claude")
	assert.NotContains(t, out, "=> copilot")
	assert.NotContains(t, out, "=> opencode")
}

func TestRunInspect_unknownTool_returnsError(t *testing.T) {
	// Arrange
	restore := stubInspectImage(t, builtInfo, nil)
	defer restore()

	// Act
	err := runInspect(inspectCmd, []string{"bogus"})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bogus")
}

func TestRunInspect_builtImage_printsAllFields(t *testing.T) {
	// Arrange
	restore := stubInspectImage(t, builtInfo, nil)
	defer restore()

	// Act
	out := captureStdout(t, func() {
		err := runInspect(inspectCmd, []string{"claude"})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "agentic-claude (a1b2c3d4e5f6)")
	assert.Contains(t, out, "1.2.3")
	assert.Contains(t, out, "node:24")
	assert.Contains(t, out, "2026-05-01")
	assert.Contains(t, out, "512 MB")
}

func TestRunInspect_notBuilt_printsFallback(t *testing.T) {
	// Arrange
	restore := stubInspectImage(t, nil, nil)
	defer restore()

	// Act
	out := captureStdout(t, func() {
		err := runInspect(inspectCmd, []string{"claude"})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "agentic-claude (not built)")
}

func TestRunInspect_emptyLabels_printsFallbacks(t *testing.T) {
	// Arrange
	restore := stubInspectImage(t, &docker.ImageInfo{
		Image:  "agentic-claude",
		ID:     "a1b2c3d4e5f6",
		SizeMB: 100,
	}, nil)
	defer restore()

	// Act
	out := captureStdout(t, func() {
		err := runInspect(inspectCmd, []string{"claude"})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "(unknown - rebuild to capture)")
	assert.Contains(t, out, "(unknown)")
}

func TestRunInspect_dockerError_propagates(t *testing.T) {
	// Arrange
	orig := inspectImage
	inspectImage = func(_ string) (*docker.ImageInfo, error) {
		return nil, fmt.Errorf("docker daemon not running")
	}
	defer func() { inspectImage = orig }()

	// Act
	err := runInspect(inspectCmd, []string{"claude"})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docker daemon not running")
}

func TestRunInspect_invalidOutputFormat_returnsError(t *testing.T) {
	// Arrange
	outputFmt = "json"
	defer func() { outputFmt = "default" }()

	// Act
	err := runInspect(inspectCmd, []string{"claude"})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "json")
}

func TestRunInspect_tableOutput_showsHeader(t *testing.T) {
	// Arrange
	restore := stubInspectImage(t, builtInfo, nil)
	defer restore()
	outputFmt = "table"
	defer func() { outputFmt = "default" }()

	// Act
	out := captureStdout(t, func() {
		err := runInspect(inspectCmd, []string{})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "TOOL")
	assert.Contains(t, out, "IMAGE")
	assert.Contains(t, out, "VERSION")
	assert.Contains(t, out, "BUILT")
	assert.Contains(t, out, "SIZE")
}

func TestRunInspect_tableOutput_builtImage_showsFields(t *testing.T) {
	// Arrange
	restore := stubInspectImage(t, builtInfo, nil)
	defer restore()
	outputFmt = "table"
	defer func() { outputFmt = "default" }()

	// Act
	out := captureStdout(t, func() {
		err := runInspect(inspectCmd, []string{"claude"})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "claude")
	assert.Contains(t, out, "1.2.3")
	assert.Contains(t, out, "2026-05-01")
	assert.Contains(t, out, "512 MB")
}

func TestRunInspect_tableOutput_notBuilt_showsDashes(t *testing.T) {
	// Arrange
	restore := stubInspectImage(t, nil, nil)
	defer restore()
	outputFmt = "table"
	defer func() { outputFmt = "default" }()

	// Act
	out := captureStdout(t, func() {
		err := runInspect(inspectCmd, []string{"claude"})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "(not built)")
}

func TestRunInspect_tableOutput_emptyLabels_showsUnknown(t *testing.T) {
	// Arrange
	restore := stubInspectImage(t, &docker.ImageInfo{Image: "agentic-claude", ID: "abc", SizeMB: 100}, nil)
	defer restore()
	outputFmt = "table"
	defer func() { outputFmt = "default" }()

	// Act
	out := captureStdout(t, func() {
		err := runInspect(inspectCmd, []string{"claude"})
		require.NoError(t, err)
	})

	// Assert
	assert.Contains(t, out, "(unknown)")
}

func TestRunInspect_tableOutput_dockerError_propagates(t *testing.T) {
	// Arrange
	orig := inspectImage
	inspectImage = func(_ string) (*docker.ImageInfo, error) {
		return nil, fmt.Errorf("docker daemon not running")
	}
	defer func() { inspectImage = orig }()
	outputFmt = "table"
	defer func() { outputFmt = "default" }()

	// Act
	err := runInspect(inspectCmd, []string{"claude"})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docker daemon not running")
}
