package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeMarketplaceRC writes a minimal .agenticrc.toml declaring one
// [[marketplaces]] entry into dir.
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
		dirName := marketplace.CloneDirName("acme", "git@example.com:acme.git")

		// Act
		live := liveMarketplaceProjects(dirName, []string{projA})

		// Assert
		assert.Equal(t, []string{projA}, live)
	})

	t.Run("drops projects whose config no longer references it", func(t *testing.T) {
		// Arrange
		projA := t.TempDir()
		writeMarketplaceRC(t, projA, "acme", "git@example.com:different.git")
		dirName := marketplace.CloneDirName("acme", "git@example.com:acme.git")

		// Act
		live := liveMarketplaceProjects(dirName, []string{projA})

		// Assert
		assert.Empty(t, live)
	})

	t.Run("drops projects whose directory no longer exists", func(t *testing.T) {
		// Arrange
		missing := filepath.Join(t.TempDir(), "gone")
		dirName := marketplace.CloneDirName("acme", "git@example.com:acme.git")

		// Act
		live := liveMarketplaceProjects(dirName, []string{missing})

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
		trackedDir := marketplace.CloneDirName("acme", "git@example.com:acme.git")
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, trackedDir), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "untracked-clone"), 0o755))
		reg := &marketplace.Registry{Marketplaces: map[string]marketplace.RegistryEntry{
			trackedDir: {Name: "acme", URL: "git@example.com:acme.git", Projects: []string{"/home/user/projA"}},
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
}

func TestRunMarketplacesPrune(t *testing.T) {
	t.Run("keeps a clone still referenced by a live project", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		baseDir := filepath.Join(toolHome, "marketplaces")
		proj := t.TempDir()
		writeMarketplaceRC(t, proj, "acme", "git@example.com:acme.git")
		dirName := marketplace.CloneDirName("acme", "git@example.com:acme.git")
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, dirName), 0o755))
		reg := &marketplace.Registry{Marketplaces: map[string]marketplace.RegistryEntry{
			dirName: {Name: "acme", URL: "git@example.com:acme.git", Projects: []string{proj}},
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
		assert.Equal(t, []string{proj}, updated.Marketplaces[dirName].Projects)
	})

	t.Run("removes a clone no project references anymore", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		baseDir := filepath.Join(toolHome, "marketplaces")
		proj := t.TempDir() // exists but no longer declares this marketplace
		dirName := marketplace.CloneDirName("acme", "git@example.com:acme.git")
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, dirName), 0o755))
		reg := &marketplace.Registry{Marketplaces: map[string]marketplace.RegistryEntry{
			dirName: {Name: "acme", URL: "git@example.com:acme.git", Projects: []string{proj}},
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
		dirName := marketplace.CloneDirName("acme", "git@example.com:acme.git")
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, dirName), 0o755))
		reg := &marketplace.Registry{Marketplaces: map[string]marketplace.RegistryEntry{
			dirName: {Name: "acme", URL: "git@example.com:acme.git", Projects: []string{projA, projB}},
		}}
		require.NoError(t, marketplace.SaveRegistry(baseDir, reg))

		// Act
		require.NoError(t, runMarketplacesPrune(marketplacesPruneCmd, nil))

		// Assert
		assert.DirExists(t, filepath.Join(baseDir, dirName))
		updated, err := marketplace.LoadRegistry(baseDir)
		require.NoError(t, err)
		assert.Equal(t, []string{projA}, updated.Marketplaces[dirName].Projects)
	})
}
