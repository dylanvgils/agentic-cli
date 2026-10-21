package marketplace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloneDirs(t *testing.T) {
	t.Run("missing baseDir is not an error", func(t *testing.T) {
		// Act
		names, err := CloneDirs(filepath.Join(t.TempDir(), "does-not-exist"))

		// Assert
		require.NoError(t, err)
		assert.Empty(t, names)
	})

	t.Run("returns sorted directory names, skipping files", func(t *testing.T) {
		// Arrange
		baseDir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "zeta-1234"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "acme-5678"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(baseDir, ".usage.json"), []byte("{}"), 0o644))

		// Act
		names, err := CloneDirs(baseDir)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"acme-5678", "zeta-1234"}, names)
	})
}
