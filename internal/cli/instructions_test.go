package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runInstructions(t *testing.T) {
	stubDocker(t, &fakeDocker{})

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
