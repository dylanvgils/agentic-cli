package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunInstructions(t *testing.T) {
	t.Run("unknown tool returns error", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())

		// Act
		err := runInstructions(instructionsCmd, []string{"bogus"})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bogus")
	})

	t.Run("prints generated content", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())

		// Act
		out := captureStdout(t, func() {
			err := runInstructions(instructionsCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "## Installed toolchains")
		assert.Contains(t, out, "## Filesystem")
	})

	t.Run("disabled via config prints notice instead of content", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		rcPath := filepath.Join(dir, ".agenticrc.toml")
		require.NoError(t, os.WriteFile(rcPath, []byte("[run.instructions]\nenabled = false\n"), 0o644))
		t.Chdir(dir)

		// Act
		out := captureStdout(t, func() {
			err := runInstructions(instructionsCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "disabled via .agenticrc.toml")
		assert.NotContains(t, out, "## Installed toolchains")
	})

	t.Run("custom instructions from config are included", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		rcPath := filepath.Join(dir, ".agenticrc.toml")
		require.NoError(t, os.WriteFile(rcPath, []byte("[run.instructions]\ncustom = \"Always run go test before finishing.\"\n"), 0o644))
		t.Chdir(dir)

		// Act
		out := captureStdout(t, func() {
			err := runInstructions(instructionsCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "Always run go test before finishing.")
	})

	t.Run("merges content already persisted under the agentic home", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		t.Chdir(t.TempDir())
		hostPath := tools.Configs["claude"].Runtime.InstructionsHostPath(toolHome)
		require.NoError(t, os.MkdirAll(filepath.Dir(hostPath), 0o750))
		require.NoError(t, os.WriteFile(hostPath, []byte("my own global notes\n"), 0o640))

		// Act
		out := captureStdout(t, func() {
			err := runInstructions(instructionsCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "my own global notes")
	})

	t.Run("invalid project config fails fast with a clear error", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		rcPath := filepath.Join(dir, ".agenticrc.toml")
		require.NoError(t, os.WriteFile(rcPath, []byte("not valid toml [[["), 0o644))
		t.Chdir(dir)

		// Act
		err := runInstructions(instructionsCmd, []string{"claude"})

		// Assert
		assert.ErrorContains(t, err, rcPath)
	})
}
