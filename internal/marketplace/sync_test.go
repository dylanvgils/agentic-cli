package marketplace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSync(t *testing.T) {
	dirForBase := func(base string) func(e Entry) string {
		return func(e Entry) string { return filepath.Join(base, e.Name) }
	}

	t.Run("clones when missing", func(t *testing.T) {
		// Arrange
		var clonedURL, clonedDir string
		stubGitClone(t, func(url, dir string) error {
			clonedURL, clonedDir = url, dir
			return nil
		})
		base := t.TempDir()
		entries := []Entry{{Name: "acme", URL: "git@example.com:acme/marketplace.git"}}

		// Act
		results, err := Sync(entries, dirForBase(base))

		// Assert
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, filepath.Join(base, "acme"), results[0].Dir)
		assert.False(t, results[0].Stale)
		assert.Equal(t, "git@example.com:acme/marketplace.git", clonedURL)
		assert.Equal(t, filepath.Join(base, "acme"), clonedDir)
	})

	t.Run("fetches and resets when present", func(t *testing.T) {
		// Arrange
		base := t.TempDir()
		dir := filepath.Join(base, "acme")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		var fetchResetDir string
		stubGitFetchReset(t, func(dir string) error {
			fetchResetDir = dir
			return nil
		})
		entries := []Entry{{Name: "acme", URL: "git@example.com:acme/marketplace.git"}}

		// Act
		results, err := Sync(entries, dirForBase(base))

		// Assert
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.False(t, results[0].Stale)
		assert.Equal(t, dir, fetchResetDir)
	})

	t.Run("fetch failure warns and keeps stale clone", func(t *testing.T) {
		// Arrange
		base := t.TempDir()
		dir := filepath.Join(base, "acme")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		stubGitFetchReset(t, func(string) error { return errors.New("stub: fetch failed") })
		entries := []Entry{{Name: "acme", URL: "git@example.com:acme/marketplace.git"}}

		// Act
		results, err := Sync(entries, dirForBase(base))

		// Assert
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.True(t, results[0].Stale)
		assert.Error(t, results[0].Warning)
		assert.Equal(t, dir, results[0].Dir)
	})

	t.Run("clone failure is a hard error and stops remaining entries", func(t *testing.T) {
		// Arrange
		base := t.TempDir()
		var cloneCalls int
		stubGitClone(t, func(string, string) error {
			cloneCalls++
			return errors.New("stub: clone failed")
		})
		entries := []Entry{
			{Name: "first", URL: "git@example.com:acme/first.git"},
			{Name: "second", URL: "git@example.com:acme/second.git"},
		}

		// Act
		results, err := Sync(entries, dirForBase(base))

		// Assert
		assert.Error(t, err)
		assert.Nil(t, results)
		assert.Equal(t, 1, cloneCalls, "second entry must not be attempted after the first hard-fails")
	})

	t.Run("duplicate name rejected", func(t *testing.T) {
		// Arrange
		entries := []Entry{
			{Name: "acme", URL: "git@example.com:acme/one.git"},
			{Name: "acme", URL: "git@example.com:acme/two.git"},
		}

		// Act
		results, err := Sync(entries, dirForBase(t.TempDir()))

		// Assert
		assert.ErrorContains(t, err, "duplicate marketplace name")
		assert.Nil(t, results)
	})

	t.Run("different names, same url, share one clone", func(t *testing.T) {
		// Arrange - clone must create dir on disk, like real git, so the second entry sees it as synced
		var cloneCalls, fetchResetCalls int
		stubGitClone(t, func(_, dir string) error {
			cloneCalls++
			return os.MkdirAll(dir, 0o755)
		})
		stubGitFetchReset(t, func(string) error {
			fetchResetCalls++
			return nil
		})
		base := t.TempDir()
		dirForURL := func(e Entry) string { return filepath.Join(base, CloneDirName(e.URL)) }
		entries := []Entry{
			{Name: "infosupport", URL: "git@example.com:acme/marketplace.git"},
			{Name: "mycompany-plugins", URL: "git@example.com:acme/marketplace.git"},
		}

		// Act
		results, err := Sync(entries, dirForURL)

		// Assert
		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.Equal(t, results[0].Dir, results[1].Dir)
		assert.Equal(t, 1, cloneCalls, "expected one clone")
		assert.Equal(t, 1, fetchResetCalls, "expected one fetch+reset - not two clones")
	})
}

func TestCloneDirName(t *testing.T) {
	t.Run("different urls produce different results", func(t *testing.T) {
		// Act
		a := CloneDirName("git@example.com:acme/one.git")
		b := CloneDirName("git@example.com:acme/two.git")

		// Assert
		assert.NotEqual(t, a, b)
		assert.True(t, strings.HasPrefix(a, "one-"))
		assert.True(t, strings.HasPrefix(b, "two-"))
	})

	t.Run("stable for repeated calls with the same input", func(t *testing.T) {
		// Act
		first := CloneDirName("git@example.com:acme/one.git")
		second := CloneDirName("git@example.com:acme/one.git")

		// Assert
		assert.Equal(t, first, second)
	})
}
