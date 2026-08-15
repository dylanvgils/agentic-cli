package marketplace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLiveProjects(t *testing.T) {
	t.Run("keeps projects that still declare a matching marketplace", func(t *testing.T) {
		// Arrange
		projA := t.TempDir()
		writeMarketplaceRC(t, projA, "acme", "git@example.com:acme.git")
		dirName := CloneDirName("git@example.com:acme.git")

		// Act
		live := LiveProjects(dirName, "acme", []string{projA})

		// Assert
		assert.Equal(t, []string{projA}, live)
	})

	t.Run("drops projects whose config no longer references it", func(t *testing.T) {
		// Arrange
		projA := t.TempDir()
		writeMarketplaceRC(t, projA, "acme", "git@example.com:different.git")
		dirName := CloneDirName("git@example.com:acme.git")

		// Act
		live := LiveProjects(dirName, "acme", []string{projA})

		// Assert
		assert.Empty(t, live)
	})

	t.Run("drops projects whose directory no longer exists", func(t *testing.T) {
		// Arrange
		missing := filepath.Join(t.TempDir(), "gone")
		dirName := CloneDirName("git@example.com:acme.git")

		// Act
		live := LiveProjects(dirName, "acme", []string{missing})

		// Assert
		assert.Empty(t, live)
	})

	t.Run("drops projects that kept the URL but renamed the marketplace's name key", func(t *testing.T) {
		// Arrange
		projA := t.TempDir()
		writeMarketplaceRC(t, projA, "renamed", "git@example.com:acme.git")
		dirName := CloneDirName("git@example.com:acme.git")

		// Act
		live := LiveProjects(dirName, "acme", []string{projA})

		// Assert
		assert.Empty(t, live)
	})
}

func TestPrune(t *testing.T) {
	t.Run("keeps a clone still referenced by a live project", func(t *testing.T) {
		// Arrange
		baseDir := t.TempDir()
		proj := t.TempDir()
		writeMarketplaceRC(t, proj, "acme", "git@example.com:acme.git")
		dirName := CloneDirName("git@example.com:acme.git")
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, dirName), 0o755))
		reg := &Registry{Marketplaces: map[string][]RegistryEntry{
			dirName: {{Name: "acme", URL: "git@example.com:acme.git", Projects: []string{proj}}},
		}}

		// Act
		updated, report, err := Prune(baseDir, reg)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, filepath.Join(baseDir, dirName))
		require.Len(t, report, 1)
		assert.Equal(t, PruneResult{Kind: PruneKept, DirName: dirName, Name: "acme", Projects: []string{proj}}, report[0])
		require.Len(t, updated.Marketplaces[dirName], 1)
		assert.Equal(t, []string{proj}, updated.Marketplaces[dirName][0].Projects)
	})

	t.Run("removes a clone no project references anymore", func(t *testing.T) {
		// Arrange
		baseDir := t.TempDir()
		proj := t.TempDir() // exists but no longer declares this marketplace
		dirName := CloneDirName("git@example.com:acme.git")
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, dirName), 0o755))
		reg := &Registry{Marketplaces: map[string][]RegistryEntry{
			dirName: {{Name: "acme", URL: "git@example.com:acme.git", Projects: []string{proj}}},
		}}

		// Act
		updated, report, err := Prune(baseDir, reg)

		// Assert
		require.NoError(t, err)
		assert.NoDirExists(t, filepath.Join(baseDir, dirName))
		require.Len(t, report, 1)
		assert.Equal(t, PruneResult{Kind: PruneRemoved, DirName: dirName, Name: "acme"}, report[0])
		assert.NotContains(t, updated.Marketplaces, dirName)
	})

	t.Run("skips a clone with no usage record", func(t *testing.T) {
		// Arrange
		baseDir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "untracked-clone"), 0o755))
		reg := &Registry{Marketplaces: map[string][]RegistryEntry{}}

		// Act
		updated, report, err := Prune(baseDir, reg)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, filepath.Join(baseDir, "untracked-clone"))
		require.Len(t, report, 1)
		assert.Equal(t, PruneResult{Kind: PruneNoRecord, DirName: "untracked-clone"}, report[0])
		assert.NotContains(t, updated.Marketplaces, "untracked-clone")
	})

	t.Run("multiple projects: kept when at least one still references it", func(t *testing.T) {
		// Arrange
		baseDir := t.TempDir()
		projA := t.TempDir()
		projB := t.TempDir() // no longer references it
		writeMarketplaceRC(t, projA, "acme", "git@example.com:acme.git")
		dirName := CloneDirName("git@example.com:acme.git")
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, dirName), 0o755))
		reg := &Registry{Marketplaces: map[string][]RegistryEntry{
			dirName: {{Name: "acme", URL: "git@example.com:acme.git", Projects: []string{projA, projB}}},
		}}

		// Act
		updated, _, err := Prune(baseDir, reg)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, filepath.Join(baseDir, dirName))
		require.Len(t, updated.Marketplaces[dirName], 1)
		assert.Equal(t, []string{projA}, updated.Marketplaces[dirName][0].Projects)
	})

	t.Run("drops a dead name-entry but keeps the shared dir when a sibling name is still live", func(t *testing.T) {
		// Arrange
		baseDir := t.TempDir()
		projFoo := t.TempDir()
		projBar := t.TempDir() // no longer declares this marketplace
		writeMarketplaceRC(t, projFoo, "foo", "git@example.com:acme.git")
		dirName := CloneDirName("git@example.com:acme.git")
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, dirName), 0o755))
		reg := &Registry{Marketplaces: map[string][]RegistryEntry{
			dirName: {
				{Name: "bar", URL: "git@example.com:acme.git", Projects: []string{projBar}},
				{Name: "foo", URL: "git@example.com:acme.git", Projects: []string{projFoo}},
			},
		}}

		// Act
		updated, report, err := Prune(baseDir, reg)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, filepath.Join(baseDir, dirName))
		assert.Contains(t, report, PruneResult{Kind: PruneKept, DirName: dirName, Name: "foo", Projects: []string{projFoo}})
		assert.Contains(t, report, PruneResult{Kind: PruneDropped, DirName: dirName, Name: "bar"})
		require.Len(t, updated.Marketplaces[dirName], 1)
		assert.Equal(t, "foo", updated.Marketplaces[dirName][0].Name)
	})

	t.Run("removes the shared dir only once every name-entry is dead", func(t *testing.T) {
		// Arrange
		baseDir := t.TempDir()
		projFoo := t.TempDir() // no longer declares this marketplace
		projBar := t.TempDir() // no longer declares this marketplace
		dirName := CloneDirName("git@example.com:acme.git")
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, dirName), 0o755))
		reg := &Registry{Marketplaces: map[string][]RegistryEntry{
			dirName: {
				{Name: "bar", URL: "git@example.com:acme.git", Projects: []string{projBar}},
				{Name: "foo", URL: "git@example.com:acme.git", Projects: []string{projFoo}},
			},
		}}

		// Act
		updated, report, err := Prune(baseDir, reg)

		// Assert
		require.NoError(t, err)
		assert.NoDirExists(t, filepath.Join(baseDir, dirName))
		assert.Contains(t, report, PruneResult{Kind: PruneRemoved, DirName: dirName, Name: "foo"})
		assert.Contains(t, report, PruneResult{Kind: PruneRemoved, DirName: dirName, Name: "bar"})
		assert.NotContains(t, updated.Marketplaces, dirName)
	})
}
