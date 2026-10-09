package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomInstallsHash(t *testing.T) {
	helm := RCCustomInstall{Name: "helm", Run: []string{"echo helm"}}

	t.Run("no installs has no hash", func(t *testing.T) {
		// Act
		hash := CustomInstallsHash(nil)

		// Assert
		assert.Empty(t, hash)
	})

	t.Run("same installs hash the same", func(t *testing.T) {
		// Act
		first := CustomInstallsHash([]RCCustomInstall{helm})
		second := CustomInstallsHash([]RCCustomInstall{{Name: "helm", Run: []string{"echo helm"}}})

		// Assert
		assert.Len(t, first, 64)
		assert.Equal(t, first, second)
	})

	t.Run("changed run commands change the hash", func(t *testing.T) {
		// Act
		before := CustomInstallsHash([]RCCustomInstall{helm})
		after := CustomInstallsHash([]RCCustomInstall{{Name: "helm", Run: []string{"echo other"}}})

		// Assert
		assert.NotEqual(t, before, after)
	})
}

func TestSaveCustomInstalls(t *testing.T) {
	t.Run("writes installs under their hash", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		installs := []RCCustomInstall{{Name: "helm", Run: []string{"echo helm"}}}

		// Act
		err := SaveCustomInstalls(home, installs)

		// Assert
		require.NoError(t, err)
		assert.FileExists(t, filepath.Join(home, CustomInstallsDirName, CustomInstallsHash(installs)+".json"))
	})

	t.Run("no installs writes nothing", func(t *testing.T) {
		// Arrange
		home := t.TempDir()

		// Act
		err := SaveCustomInstalls(home, nil)

		// Assert
		require.NoError(t, err)
		assert.NoDirExists(t, filepath.Join(home, CustomInstallsDirName))
	})
}

func TestLoadCustomInstalls(t *testing.T) {
	installs := []RCCustomInstall{{Name: "helm", Run: []string{"echo helm"}}}
	hash := CustomInstallsHash(installs)

	t.Run("round-trips saved installs", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		require.NoError(t, SaveCustomInstalls(home, installs))

		// Act
		loaded, ok := LoadCustomInstalls(home, hash)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, installs, loaded)
	})

	t.Run("missing file is not ok", func(t *testing.T) {
		// Act
		_, ok := LoadCustomInstalls(t.TempDir(), hash)

		// Assert
		assert.False(t, ok)
	})

	t.Run("malformed hash is not read", func(t *testing.T) {
		// Act
		_, ok := LoadCustomInstalls(t.TempDir(), "../agentic")

		// Assert
		assert.False(t, ok)
	})

	t.Run("content not matching its hash is not ok", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		require.NoError(t, SaveCustomInstalls(home, installs))
		path := filepath.Join(home, CustomInstallsDirName, hash+".json")
		require.NoError(t, os.WriteFile(path, []byte(`[{"name":"helm","run":["echo edited"]}]`), 0o640))

		// Act
		_, ok := LoadCustomInstalls(home, hash)

		// Assert
		assert.False(t, ok)
	})
}

func TestRemoveCustomInstalls(t *testing.T) {
	installs := []RCCustomInstall{{Name: "helm", Run: []string{"echo helm"}}}
	hash := CustomInstallsHash(installs)

	t.Run("deletes the stored file", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		require.NoError(t, SaveCustomInstalls(home, installs))

		// Act
		err := RemoveCustomInstalls(home, hash)

		// Assert
		require.NoError(t, err)
		assert.NoFileExists(t, filepath.Join(home, CustomInstallsDirName, hash+".json"))
	})

	t.Run("missing file is a no-op", func(t *testing.T) {
		// Act
		err := RemoveCustomInstalls(t.TempDir(), hash)

		// Assert
		require.NoError(t, err)
	})

	t.Run("malformed hash touches nothing", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		keep := filepath.Join(home, "agentic.json")
		require.NoError(t, os.WriteFile(keep, []byte("{}"), 0o640))

		// Act
		err := RemoveCustomInstalls(filepath.Join(home, CustomInstallsDirName), "../agentic")

		// Assert
		require.NoError(t, err)
		assert.FileExists(t, keep)
	})
}
