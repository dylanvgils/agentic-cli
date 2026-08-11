package marketplace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSync(t *testing.T) {
	dirForBase := func(base string) func(name string) string {
		return func(name string) string { return filepath.Join(base, name) }
	}

	t.Run("clones when missing", func(t *testing.T) {
		// Arrange
		get := stubRunGitCapture(t)
		base := t.TempDir()
		entries := []Entry{{Name: "acme", URL: "git@example.com:acme/marketplace.git"}}

		// Act
		results, err := Sync(entries, dirForBase(base))

		// Assert
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, filepath.Join(base, "acme"), results[0].Dir)
		assert.False(t, results[0].Stale)
		calls := get()
		require.Len(t, calls, 1)
		assert.Equal(t, "clone", calls[0].args[0])
		assert.Contains(t, calls[0].args, "git@example.com:acme/marketplace.git")
	})

	t.Run("fetches and resets when present", func(t *testing.T) {
		// Arrange
		base := t.TempDir()
		dir := filepath.Join(base, "acme")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		get := stubRunGitCapture(t)
		entries := []Entry{{Name: "acme", URL: "git@example.com:acme/marketplace.git"}}

		// Act
		results, err := Sync(entries, dirForBase(base))

		// Assert
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.False(t, results[0].Stale)
		calls := get()
		require.Len(t, calls, 2)
		assert.Equal(t, []string{"-C", dir, "fetch"}, calls[0].args)
		assert.Equal(t, []string{"-C", dir, "reset", "--hard", "@{upstream}"}, calls[1].args)
	})

	t.Run("fetch failure warns and keeps stale clone", func(t *testing.T) {
		// Arrange
		base := t.TempDir()
		dir := filepath.Join(base, "acme")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		stubRunGitCapture(t, "fetch")
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
		get := stubRunGitCapture(t, "clone")
		entries := []Entry{
			{Name: "first", URL: "git@example.com:acme/first.git"},
			{Name: "second", URL: "git@example.com:acme/second.git"},
		}

		// Act
		results, err := Sync(entries, dirForBase(base))

		// Assert
		assert.Error(t, err)
		assert.Nil(t, results)
		assert.Len(t, get(), 1, "second entry must not be attempted after the first hard-fails")
	})

	t.Run("timeout treated as failure", func(t *testing.T) {
		// Arrange
		stubGitTimeout(t, time.Millisecond)
		stubRunGit(t, func(ctx context.Context, _ ...string) ([]byte, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		})
		base := t.TempDir()
		entries := []Entry{{Name: "acme", URL: "git@example.com:acme/marketplace.git"}}

		// Act
		results, err := Sync(entries, dirForBase(base))

		// Assert - nothing on disk yet, so a timed-out clone is a hard failure
		assert.Error(t, err)
		assert.Nil(t, results)
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
}

func TestPrune(t *testing.T) {
	t.Run("missing baseDir is not an error", func(t *testing.T) {
		// Arrange
		baseDir := filepath.Join(t.TempDir(), "does-not-exist")

		// Act
		removed, err := Prune(baseDir, nil)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, removed)
	})

	t.Run("removes directories not in keep", func(t *testing.T) {
		// Arrange
		baseDir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "a"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "b"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "c"), 0o755))

		// Act
		removed, err := Prune(baseDir, []string{"a", "c"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"b"}, removed)
		assert.DirExists(t, filepath.Join(baseDir, "a"))
		assert.NoDirExists(t, filepath.Join(baseDir, "b"))
		assert.DirExists(t, filepath.Join(baseDir, "c"))
	})
}

func TestCheckGitAvailable(t *testing.T) {
	t.Run("returns nil when git is on PATH", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "git"), []byte("#!/bin/sh\n"), 0o755))
		t.Setenv("PATH", dir)

		// Act
		err := CheckGitAvailable()

		// Assert
		assert.NoError(t, err)
	})

	t.Run("returns error when git is not on PATH", func(t *testing.T) {
		// Arrange
		t.Setenv("PATH", t.TempDir())

		// Act
		err := CheckGitAvailable()

		// Assert
		assert.ErrorContains(t, err, "git not found on PATH")
	})
}
