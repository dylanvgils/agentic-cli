package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDocker writes a shell script named "docker" to a temp dir and prepends
// it to PATH. t.Setenv handles cleanup automatically.
func fakeDocker(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "docker")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\n"+script+"\n"), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestRunCmd_capturesOutput(t *testing.T) {
	// Arrange
	fakeDocker(t, `printf '%s\n' "$@"`)

	// Act
	out, err := RunCmd("images", "--quiet")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "images\n--quiet\n", out)
}

func TestRunCmd_returnsErrorOnFailure(t *testing.T) {
	// Arrange
	fakeDocker(t, `exit 1`)

	// Act
	_, err := RunCmd("bad-command")

	// Assert
	assert.Error(t, err)
}

func TestRun_pipesStdin(t *testing.T) {
	// Arrange
	fakeDocker(t, `cat`)

	// Act
	out, err := Run(strings.NewReader("hello\n"), "build", "-")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "hello\n", out)
}

func TestRun_nilStdinDoesNotBlock(t *testing.T) {
	// Arrange
	fakeDocker(t, `echo ok`)

	// Act
	out, err := Run(nil, "info")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "ok\n", out)
}
