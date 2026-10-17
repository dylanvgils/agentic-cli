package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runInstructions(t *testing.T) {
	// Arrange
	stubDocker(t, &fakeDocker{})
	t.Chdir(t.TempDir())

	// Act
	var err error
	out := captureStdout(t, func() {
		err = runInstructions(instructionsCmd, []string{"claude"})
	})

	// Assert
	require.NoError(t, err)
	assert.Contains(t, out, "## Installed toolchains")
	assert.Contains(t, out, "## Filesystem")
}

func Test_instructionsRequest(t *testing.T) {
	t.Run("request targets the tool's image in the resolved namespace", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())

		// Act
		req, namespace, err := instructionsRequest(instructionsCmd, "claude")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "claude", req.Target.ToolName)
		assert.Contains(t, req.Target.ImageName, "claude")
		assert.NotEmpty(t, namespace)
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())

		// Act
		_, _, err := instructionsRequest(instructionsCmd, "bogus")

		// Assert
		assert.ErrorContains(t, err, "bogus")
	})

	t.Run("runtime flag lands in the request", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		stubFlag(t, instructionsCmd, "runtime", "microsandbox")

		// Act
		req, _, err := instructionsRequest(instructionsCmd, "claude")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, sandbox.RuntimeMicrosandbox, req.Flags.Runtime)
	})

	t.Run("unknown runtime returns error", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		stubFlag(t, instructionsCmd, "runtime", "podman")

		// Act
		_, _, err := instructionsRequest(instructionsCmd, "claude")

		// Assert
		assert.ErrorContains(t, err, `unknown runtime "podman"`)
	})

	t.Run("invalid project config fails fast with a clear error", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		rcPath := filepath.Join(dir, ".agenticrc.toml")
		require.NoError(t, os.WriteFile(rcPath, []byte("not valid toml [[["), 0o644))
		t.Chdir(dir)

		// Act
		_, _, err := instructionsRequest(instructionsCmd, "claude")

		// Assert
		assert.ErrorContains(t, err, rcPath)
	})
}

func Test_printInstructions(t *testing.T) {
	t.Run("content is printed under its heading", func(t *testing.T) {
		// Act
		var logs string
		out := captureStdout(t, func() {
			logs = captureLog(t, func() { printInstructions("agentic/claude", "## Filesystem\n") })
		})

		// Assert
		assert.Contains(t, logs, "agentic/claude")
		assert.Equal(t, "## Filesystem\n", out)
	})

	t.Run("empty content prints the disabled notice", func(t *testing.T) {
		// Act
		logs := captureLog(t, func() { printInstructions("agentic/claude", "") })

		// Assert
		assert.Contains(t, logs, "disabled via .agenticrc.toml")
	})
}
