package docker

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- RunCmd ---
func TestRunCmd_delegatesToRun(t *testing.T) {
	// Arrange
	var capturedArgs []string
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		capturedArgs = args
		return "ok", nil
	}
	defer func() { dockerRun = orig }()

	// Act
	out, err := dockerRun("images", "--quiet")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "ok", out)
	assert.Equal(t, []string{"images", "--quiet"}, capturedArgs)
}

// --- dockerRunStdin ---
func TestDockerRunStdin_passesReaderAndArgs(t *testing.T) {
	// Arrange
	var capturedReader io.Reader
	var capturedArgs []string
	orig := dockerRunStdin
	dockerRunStdin = func(r io.Reader, args ...string) (string, error) {
		capturedReader = r
		capturedArgs = args
		return "", nil
	}
	defer func() { dockerRunStdin = orig }()

	reader := strings.NewReader("FROM scratch\n")

	// Act
	_, err := dockerRunStdin(reader, "build", "--quiet", "-")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, reader, capturedReader)
	assert.Equal(t, []string{"build", "--quiet", "-"}, capturedArgs)
}
