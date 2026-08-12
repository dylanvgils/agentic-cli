package marketplace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRegistry(t *testing.T) {
	t.Run("missing file returns empty registry", func(t *testing.T) {
		// Act
		reg, err := LoadRegistry(t.TempDir())

		// Assert
		require.NoError(t, err)
		assert.Empty(t, reg.Marketplaces)
	})

	t.Run("round-trips through save", func(t *testing.T) {
		// Arrange
		baseDir := t.TempDir()
		reg := &Registry{Marketplaces: map[string][]RegistryEntry{
			"acme-abcd1234": {{Name: "acme", URL: "git@example.com:acme.git", Projects: []string{"/home/user/proj"}}},
		}}
		require.NoError(t, SaveRegistry(baseDir, reg))

		// Act
		loaded, err := LoadRegistry(baseDir)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, reg.Marketplaces, loaded.Marketplaces)
	})

	t.Run("creates baseDir on save if missing", func(t *testing.T) {
		// Arrange
		baseDir := filepath.Join(t.TempDir(), "nested", "marketplaces")
		reg := &Registry{Marketplaces: map[string][]RegistryEntry{}}

		// Act
		err := SaveRegistry(baseDir, reg)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, baseDir)
	})

	t.Run("old single-entry-per-key format falls back to an empty registry", func(t *testing.T) {
		// Arrange
		baseDir := t.TempDir()
		old := `{"marketplaces":{"acme-abcd1234":{"name":"acme","url":"git@example.com:acme.git","projects":["/home/user/proj"]}}}`
		require.NoError(t, os.WriteFile(filepath.Join(baseDir, ".usage.json"), []byte(old), 0o644))

		// Act
		reg, err := LoadRegistry(baseDir)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, reg.Marketplaces)
	})
}

func TestRegistry_Record(t *testing.T) {
	t.Run("adds a new entry", func(t *testing.T) {
		// Arrange
		reg := &Registry{Marketplaces: map[string][]RegistryEntry{}}

		// Act
		reg.Record("acme-abcd1234", RegistryEntry{Name: "acme", URL: "git@example.com:acme.git"}, "/home/user/projA")

		// Assert
		entries := reg.Marketplaces["acme-abcd1234"]
		require.Len(t, entries, 1)
		assert.Equal(t, "acme", entries[0].Name)
		assert.Equal(t, []string{"/home/user/projA"}, entries[0].Projects)
	})

	t.Run("dedupes project dirs across calls", func(t *testing.T) {
		// Arrange
		reg := &Registry{Marketplaces: map[string][]RegistryEntry{}}
		reg.Record("acme-abcd1234", RegistryEntry{Name: "acme", URL: "git@example.com:acme.git"}, "/home/user/projA")

		// Act
		reg.Record("acme-abcd1234", RegistryEntry{Name: "acme", URL: "git@example.com:acme.git"}, "/home/user/projA")
		reg.Record("acme-abcd1234", RegistryEntry{Name: "acme", URL: "git@example.com:acme.git"}, "/home/user/projB")

		// Assert
		entries := reg.Marketplaces["acme-abcd1234"]
		require.Len(t, entries, 1)
		assert.Equal(t, []string{"/home/user/projA", "/home/user/projB"}, entries[0].Projects)
	})

	t.Run("refreshes Stale on each call", func(t *testing.T) {
		// Arrange
		reg := &Registry{Marketplaces: map[string][]RegistryEntry{}}
		reg.Record("acme-abcd1234", RegistryEntry{Name: "acme", URL: "git@example.com:acme.git", Stale: true}, "/home/user/projA")

		// Act
		reg.Record("acme-abcd1234", RegistryEntry{Name: "acme", URL: "git@example.com:acme.git", Stale: false}, "/home/user/projA")

		// Assert
		entries := reg.Marketplaces["acme-abcd1234"]
		require.Len(t, entries, 1)
		assert.False(t, entries[0].Stale)
	})

	t.Run("keeps separate entries for different names under the same key", func(t *testing.T) {
		// Arrange
		reg := &Registry{Marketplaces: map[string][]RegistryEntry{}}

		// Act
		reg.Record("acme-abcd1234", RegistryEntry{Name: "foo", URL: "git@example.com:acme.git"}, "/home/user/projA")
		reg.Record("acme-abcd1234", RegistryEntry{Name: "bar", URL: "git@example.com:acme.git"}, "/home/user/projB")

		// Assert
		entries := reg.Marketplaces["acme-abcd1234"]
		require.Len(t, entries, 2)
		assert.Equal(t, "bar", entries[0].Name)
		assert.Equal(t, []string{"/home/user/projB"}, entries[0].Projects)
		assert.Equal(t, "foo", entries[1].Name)
		assert.Equal(t, []string{"/home/user/projA"}, entries[1].Projects)
	})
}
