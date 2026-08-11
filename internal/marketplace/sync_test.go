package marketplace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSync(t *testing.T) {
	dirForBase := func(base string) func(e Entry) string {
		return func(e Entry) string { return filepath.Join(base, e.Name) }
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

func TestCloneDirName(t *testing.T) {
	t.Run("same name different url produces different results", func(t *testing.T) {
		// Act
		a := CloneDirName("acme", "git@example.com:acme/one.git")
		b := CloneDirName("acme", "git@example.com:acme/two.git")

		// Assert
		assert.NotEqual(t, a, b)
		assert.True(t, strings.HasPrefix(a, "acme-"))
		assert.True(t, strings.HasPrefix(b, "acme-"))
	})

	t.Run("stable for repeated calls with the same inputs", func(t *testing.T) {
		// Act
		first := CloneDirName("acme", "git@example.com:acme/one.git")
		second := CloneDirName("acme", "git@example.com:acme/one.git")

		// Assert
		assert.Equal(t, first, second)
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
