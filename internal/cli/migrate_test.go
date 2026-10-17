package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runMigrate(t *testing.T) {
	t.Run("prints already up to date", func(t *testing.T) {
		// Arrange
		home := filepath.Join(t.TempDir(), "home")
		stubToolHome(t, home)
		logBuf := stubErrLog(t)

		// Act
		err := runMigrate(migrateCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.FileExists(t, filepath.Join(home, ".migrations.json"))
		assert.Contains(t, logBuf.String(), "agentic: already up to date")
	})

	t.Run("propagates the migration error", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(home, ".migrations.json"), []byte("not json"), 0o644))
		stubToolHome(t, home)

		// Act
		err := runMigrate(migrateCmd, nil)

		// Assert
		assert.ErrorContains(t, err, "invalid character")
	})
}
