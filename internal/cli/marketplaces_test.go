package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeMarketplaceRC writes a minimal .agenticrc.toml declaring one marketplace into dir.
func writeMarketplaceRC(t *testing.T, dir, name, url string) {
	t.Helper()
	content := "root = true\n\n[[marketplaces]]\nname = \"" + name + "\"\nurl = \"" + url + "\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".agenticrc.toml"), []byte(content), 0o644))
}

func TestMarketplaceCloneDirs(t *testing.T) {
	t.Run("missing baseDir is not an error", func(t *testing.T) {
		// Act
		names, err := marketplaceCloneDirs(filepath.Join(t.TempDir(), "does-not-exist"))

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
		names, err := marketplaceCloneDirs(baseDir)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"acme-5678", "zeta-1234"}, names)
	})
}

func TestFormatProjects(t *testing.T) {
	t.Run("empty returns untracked", func(t *testing.T) {
		// Act
		out := formatProjects(nil)

		// Assert
		assert.Equal(t, "(untracked)", out)
	})

	t.Run("flags a project dir that no longer exists", func(t *testing.T) {
		// Arrange
		existing := t.TempDir()
		missing := filepath.Join(t.TempDir(), "gone")

		// Act
		out := formatProjects([]string{existing, missing})

		// Assert
		assert.Contains(t, out, existing)
		assert.Contains(t, out, missing+" (missing)")
	})
}

func TestLiveMarketplaceProjects(t *testing.T) {
	t.Run("keeps projects that still declare a matching marketplace", func(t *testing.T) {
		// Arrange
		projA := t.TempDir()
		writeMarketplaceRC(t, projA, "acme", "git@example.com:acme.git")
		dirName := marketplace.CloneDirName("git@example.com:acme.git")

		// Act
		live := liveMarketplaceProjects(dirName, "acme", []string{projA})

		// Assert
		assert.Equal(t, []string{projA}, live)
	})

	t.Run("drops projects whose config no longer references it", func(t *testing.T) {
		// Arrange
		projA := t.TempDir()
		writeMarketplaceRC(t, projA, "acme", "git@example.com:different.git")
		dirName := marketplace.CloneDirName("git@example.com:acme.git")

		// Act
		live := liveMarketplaceProjects(dirName, "acme", []string{projA})

		// Assert
		assert.Empty(t, live)
	})

	t.Run("drops projects whose directory no longer exists", func(t *testing.T) {
		// Arrange
		missing := filepath.Join(t.TempDir(), "gone")
		dirName := marketplace.CloneDirName("git@example.com:acme.git")

		// Act
		live := liveMarketplaceProjects(dirName, "acme", []string{missing})

		// Assert
		assert.Empty(t, live)
	})

	t.Run("drops projects that kept the URL but renamed the marketplace's name key", func(t *testing.T) {
		// Arrange
		projA := t.TempDir()
		writeMarketplaceRC(t, projA, "renamed", "git@example.com:acme.git")
		dirName := marketplace.CloneDirName("git@example.com:acme.git")

		// Act
		live := liveMarketplaceProjects(dirName, "acme", []string{projA})

		// Assert
		assert.Empty(t, live)
	})
}

func TestRunMarketplacesList(t *testing.T) {
	t.Run("no clones found", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)

		// Act
		out := captureStdout(t, func() {
			require.NoError(t, runMarketplacesList(marketplacesListCmd, nil))
		})

		// Assert
		assert.Contains(t, out, "No synced marketplaces found.")
	})

	t.Run("lists tracked and untracked clones", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		baseDir := filepath.Join(toolHome, "marketplaces")
		trackedDir := marketplace.CloneDirName("git@example.com:acme.git")
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, trackedDir), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "untracked-clone"), 0o755))
		reg := &marketplace.Registry{Marketplaces: map[string][]marketplace.RegistryEntry{
			trackedDir: {{Name: "acme", URL: "git@example.com:acme.git", Projects: []string{"/home/user/projA"}}},
		}}
		require.NoError(t, marketplace.SaveRegistry(baseDir, reg))

		// Act
		out := captureStdout(t, func() {
			require.NoError(t, runMarketplacesList(marketplacesListCmd, nil))
		})

		// Assert
		assert.Contains(t, out, "acme")
		assert.Contains(t, out, "git@example.com:acme.git")
		assert.Contains(t, out, "/home/user/projA")
		assert.Contains(t, out, "untracked-clone")
		assert.Contains(t, out, "(untracked)")
	})

	t.Run("lists multiple names sharing one clone dir", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		baseDir := filepath.Join(toolHome, "marketplaces")
		trackedDir := marketplace.CloneDirName("git@example.com:acme.git")
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, trackedDir), 0o755))
		reg := &marketplace.Registry{Marketplaces: map[string][]marketplace.RegistryEntry{
			trackedDir: {
				{Name: "bar", URL: "git@example.com:acme.git", Projects: []string{"/home/user/projB"}},
				{Name: "foo", URL: "git@example.com:acme.git", Projects: []string{"/home/user/projA"}},
			},
		}}
		require.NoError(t, marketplace.SaveRegistry(baseDir, reg))

		// Act
		out := captureStdout(t, func() {
			require.NoError(t, runMarketplacesList(marketplacesListCmd, nil))
		})

		// Assert
		assert.Contains(t, out, "foo")
		assert.Contains(t, out, "bar")
		assert.Contains(t, out, "/home/user/projA")
		assert.Contains(t, out, "/home/user/projB")
		assert.Equal(t, 2, strings.Count(out, "git@example.com:acme.git"))
	})
}

func TestRunMarketplacesPrune(t *testing.T) {
	t.Run("keeps a clone still referenced by a live project", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		baseDir := filepath.Join(toolHome, "marketplaces")
		proj := t.TempDir()
		writeMarketplaceRC(t, proj, "acme", "git@example.com:acme.git")
		dirName := marketplace.CloneDirName("git@example.com:acme.git")
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, dirName), 0o755))
		reg := &marketplace.Registry{Marketplaces: map[string][]marketplace.RegistryEntry{
			dirName: {{Name: "acme", URL: "git@example.com:acme.git", Projects: []string{proj}}},
		}}
		require.NoError(t, marketplace.SaveRegistry(baseDir, reg))

		// Act
		out := captureStdout(t, func() {
			require.NoError(t, runMarketplacesPrune(marketplacesPruneCmd, nil))
		})

		// Assert
		assert.DirExists(t, filepath.Join(baseDir, dirName))
		assert.Contains(t, out, "kept: acme")
		updated, err := marketplace.LoadRegistry(baseDir)
		require.NoError(t, err)
		require.Len(t, updated.Marketplaces[dirName], 1)
		assert.Equal(t, []string{proj}, updated.Marketplaces[dirName][0].Projects)
	})

	t.Run("removes a clone no project references anymore", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		baseDir := filepath.Join(toolHome, "marketplaces")
		proj := t.TempDir() // exists but no longer declares this marketplace
		dirName := marketplace.CloneDirName("git@example.com:acme.git")
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, dirName), 0o755))
		reg := &marketplace.Registry{Marketplaces: map[string][]marketplace.RegistryEntry{
			dirName: {{Name: "acme", URL: "git@example.com:acme.git", Projects: []string{proj}}},
		}}
		require.NoError(t, marketplace.SaveRegistry(baseDir, reg))

		// Act
		out := captureStdout(t, func() {
			require.NoError(t, runMarketplacesPrune(marketplacesPruneCmd, nil))
		})

		// Assert
		assert.NoDirExists(t, filepath.Join(baseDir, dirName))
		assert.Contains(t, out, "removed: acme")
		updated, err := marketplace.LoadRegistry(baseDir)
		require.NoError(t, err)
		assert.NotContains(t, updated.Marketplaces, dirName)
	})

	t.Run("skips a clone with no usage record", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		baseDir := filepath.Join(toolHome, "marketplaces")
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "untracked-clone"), 0o755))

		// Act
		out := captureStdout(t, func() {
			require.NoError(t, runMarketplacesPrune(marketplacesPruneCmd, nil))
		})

		// Assert
		assert.DirExists(t, filepath.Join(baseDir, "untracked-clone"))
		assert.Contains(t, out, "no usage record")
	})

	t.Run("multiple projects: kept when at least one still references it", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		baseDir := filepath.Join(toolHome, "marketplaces")
		projA := t.TempDir()
		projB := t.TempDir() // no longer references it
		writeMarketplaceRC(t, projA, "acme", "git@example.com:acme.git")
		dirName := marketplace.CloneDirName("git@example.com:acme.git")
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, dirName), 0o755))
		reg := &marketplace.Registry{Marketplaces: map[string][]marketplace.RegistryEntry{
			dirName: {{Name: "acme", URL: "git@example.com:acme.git", Projects: []string{projA, projB}}},
		}}
		require.NoError(t, marketplace.SaveRegistry(baseDir, reg))

		// Act
		require.NoError(t, runMarketplacesPrune(marketplacesPruneCmd, nil))

		// Assert
		assert.DirExists(t, filepath.Join(baseDir, dirName))
		updated, err := marketplace.LoadRegistry(baseDir)
		require.NoError(t, err)
		require.Len(t, updated.Marketplaces[dirName], 1)
		assert.Equal(t, []string{projA}, updated.Marketplaces[dirName][0].Projects)
	})

	t.Run("drops a dead name-entry but keeps the shared dir when a sibling name is still live", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		baseDir := filepath.Join(toolHome, "marketplaces")
		projFoo := t.TempDir()
		projBar := t.TempDir() // no longer declares this marketplace
		writeMarketplaceRC(t, projFoo, "foo", "git@example.com:acme.git")
		dirName := marketplace.CloneDirName("git@example.com:acme.git")
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, dirName), 0o755))
		reg := &marketplace.Registry{Marketplaces: map[string][]marketplace.RegistryEntry{
			dirName: {
				{Name: "bar", URL: "git@example.com:acme.git", Projects: []string{projBar}},
				{Name: "foo", URL: "git@example.com:acme.git", Projects: []string{projFoo}},
			},
		}}
		require.NoError(t, marketplace.SaveRegistry(baseDir, reg))

		// Act
		out := captureStdout(t, func() {
			require.NoError(t, runMarketplacesPrune(marketplacesPruneCmd, nil))
		})

		// Assert
		assert.DirExists(t, filepath.Join(baseDir, dirName))
		assert.Contains(t, out, "kept: foo")
		assert.Contains(t, out, "dropped: bar")
		updated, err := marketplace.LoadRegistry(baseDir)
		require.NoError(t, err)
		require.Len(t, updated.Marketplaces[dirName], 1)
		assert.Equal(t, "foo", updated.Marketplaces[dirName][0].Name)
	})

	t.Run("removes the shared dir only once every name-entry is dead", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		baseDir := filepath.Join(toolHome, "marketplaces")
		projFoo := t.TempDir() // no longer declares this marketplace
		projBar := t.TempDir() // no longer declares this marketplace
		dirName := marketplace.CloneDirName("git@example.com:acme.git")
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, dirName), 0o755))
		reg := &marketplace.Registry{Marketplaces: map[string][]marketplace.RegistryEntry{
			dirName: {
				{Name: "bar", URL: "git@example.com:acme.git", Projects: []string{projBar}},
				{Name: "foo", URL: "git@example.com:acme.git", Projects: []string{projFoo}},
			},
		}}
		require.NoError(t, marketplace.SaveRegistry(baseDir, reg))

		// Act
		out := captureStdout(t, func() {
			require.NoError(t, runMarketplacesPrune(marketplacesPruneCmd, nil))
		})

		// Assert
		assert.NoDirExists(t, filepath.Join(baseDir, dirName))
		assert.Contains(t, out, "removed: foo")
		assert.Contains(t, out, "removed: bar")
		updated, err := marketplace.LoadRegistry(baseDir)
		require.NoError(t, err)
		assert.NotContains(t, updated.Marketplaces, dirName)
	})

	t.Run("drops a name-entry when the project renamed the marketplace but kept the URL", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		baseDir := filepath.Join(toolHome, "marketplaces")
		proj := t.TempDir()
		writeMarketplaceRC(t, proj, "renamed", "git@example.com:acme.git") // was "acme", renamed in place
		dirName := marketplace.CloneDirName("git@example.com:acme.git")
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, dirName), 0o755))
		reg := &marketplace.Registry{Marketplaces: map[string][]marketplace.RegistryEntry{
			dirName: {{Name: "acme", URL: "git@example.com:acme.git", Projects: []string{proj}}},
		}}
		require.NoError(t, marketplace.SaveRegistry(baseDir, reg))

		// Act
		out := captureStdout(t, func() {
			require.NoError(t, runMarketplacesPrune(marketplacesPruneCmd, nil))
		})

		// Assert
		assert.NoDirExists(t, filepath.Join(baseDir, dirName))
		assert.Contains(t, out, "removed: acme")
		updated, err := marketplace.LoadRegistry(baseDir)
		require.NoError(t, err)
		assert.NotContains(t, updated.Marketplaces, dirName)
	})
}
