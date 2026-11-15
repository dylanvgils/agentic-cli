package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRunID is the run ID of test runs.
const testRunID RunID = "0123456789ab"

func TestRunTempDir(t *testing.T) {
	// Arrange
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)

	// Act
	dir := RunTempDir(testRunID)

	// Assert
	assert.Equal(t, filepath.Join(tmp, "agentic-run-0123456789ab"), dir)
}

func TestCreateRunTempDir(t *testing.T) {
	t.Run("creates a dir only this user can open", func(t *testing.T) {
		// Arrange
		t.Setenv("TMPDIR", t.TempDir())

		// Act
		err := CreateRunTempDir(testRunID)

		// Assert
		require.NoError(t, err)
		info, err := os.Stat(RunTempDir(testRunID))
		require.NoError(t, err)
		assert.True(t, info.IsDir())
		assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	})

	t.Run("refuses a dir that already exists", func(t *testing.T) {
		// Arrange
		t.Setenv("TMPDIR", t.TempDir())
		require.NoError(t, os.Mkdir(RunTempDir(testRunID), 0o777))

		// Act
		err := CreateRunTempDir(testRunID)

		// Assert
		require.ErrorIs(t, err, os.ErrExist)
	})

	t.Run("refuses an invalid run id", func(t *testing.T) {
		// Act
		err := CreateRunTempDir("../x")

		// Assert
		require.EqualError(t, err, `invalid run id "../x"`)
	})
}

func TestCreateRunFile(t *testing.T) {
	t.Run("creates a file only this user can read", func(t *testing.T) {
		// Arrange
		t.Setenv("TMPDIR", t.TempDir())
		require.NoError(t, CreateRunTempDir(testRunID))

		// Act
		f, err := CreateRunFile(testRunID, "example.env")

		// Assert
		require.NoError(t, err)
		defer f.Close() //nolint:errcheck
		assert.Equal(t, filepath.Join(RunTempDir(testRunID), "example.env"), f.Name())
		info, err := f.Stat()
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("refuses a file that already exists", func(t *testing.T) {
		// Arrange
		t.Setenv("TMPDIR", t.TempDir())
		require.NoError(t, CreateRunTempDir(testRunID))
		require.NoError(t, os.WriteFile(filepath.Join(RunTempDir(testRunID), "example.env"), nil, 0o600))

		// Act
		_, err := CreateRunFile(testRunID, "example.env")

		// Assert
		require.ErrorIs(t, err, os.ErrExist)
	})
}

func TestRemoveRunTempDir(t *testing.T) {
	t.Run("removes the dir with its files", func(t *testing.T) {
		// Arrange
		t.Setenv("TMPDIR", t.TempDir())
		require.NoError(t, CreateRunTempDir(testRunID))
		require.NoError(t, os.WriteFile(filepath.Join(RunTempDir(testRunID), "example.env"), nil, 0o600))

		// Act
		err := RemoveRunTempDir(testRunID)

		// Assert
		require.NoError(t, err)
		assert.NoDirExists(t, RunTempDir(testRunID))
	})

	t.Run("a missing dir is fine", func(t *testing.T) {
		// Arrange
		t.Setenv("TMPDIR", t.TempDir())

		// Act
		err := RemoveRunTempDir(testRunID)

		// Assert
		require.NoError(t, err)
	})

	t.Run("refuses an invalid run id", func(t *testing.T) {
		// Act
		err := RemoveRunTempDir("../x")

		// Assert
		require.EqualError(t, err, `invalid run id "../x"`)
	})
}
