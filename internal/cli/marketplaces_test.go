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
	t.Run("loads, prunes, prints one message per outcome kind, and persists the result", func(t *testing.T) {
		// Arrange: dirKept holds a live "foo" alongside a dead "bar" (PruneKept + PruneDropped);
		// dirRemoved holds only a dead entry (PruneRemoved); dirUntracked has no registry entry (PruneNoRecord).
		withTempToolHome(t)
		baseDir := filepath.Join(toolHome, "marketplaces")
		projFoo := t.TempDir()
		projBar := t.TempDir() // no longer declares this marketplace
		writeMarketplaceRC(t, projFoo, "foo", "git@example.com:acme.git")
		dirKept := marketplace.CloneDirName("git@example.com:acme.git")
		dirRemoved := marketplace.CloneDirName("git@example.com:gone.git")
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, dirKept), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, dirRemoved), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "dirUntracked"), 0o755))
		reg := &marketplace.Registry{Marketplaces: map[string][]marketplace.RegistryEntry{
			dirKept: {
				{Name: "bar", URL: "git@example.com:acme.git", Projects: []string{projBar}},
				{Name: "foo", URL: "git@example.com:acme.git", Projects: []string{projFoo}},
			},
			dirRemoved: {{Name: "gone", URL: "git@example.com:gone.git", Projects: []string{projBar}}},
		}}
		require.NoError(t, marketplace.SaveRegistry(baseDir, reg))

		// Act
		out := captureLog(t, func() {
			require.NoError(t, runMarketplacesPrune(marketplacesPruneCmd, nil))
		})

		// Assert: each PruneKind's message text is wired correctly
		assert.Contains(t, out, "kept: foo (used by "+projFoo+")")
		assert.Contains(t, out, "dropped: bar (no project references it")
		assert.Contains(t, out, "removed: gone (no project references it)")
		assert.Contains(t, out, "dirUntracked: no usage record")

		// Assert: filesystem and registry reflect Prune's decisions
		assert.DirExists(t, filepath.Join(baseDir, dirKept))
		assert.NoDirExists(t, filepath.Join(baseDir, dirRemoved))
		assert.DirExists(t, filepath.Join(baseDir, "dirUntracked"))
		updated, err := marketplace.LoadRegistry(baseDir)
		require.NoError(t, err)
		require.Len(t, updated.Marketplaces[dirKept], 1)
		assert.Equal(t, "foo", updated.Marketplaces[dirKept][0].Name)
		assert.NotContains(t, updated.Marketplaces, dirRemoved)
	})
}
