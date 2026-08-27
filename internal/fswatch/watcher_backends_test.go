// Exercises the common Watcher behavior that both inotify (Linux) and kqueue (macOS) must satisfy.
//go:build linux || darwin

package fswatch

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatcher(t *testing.T) {
	t.Run("detects a write under the watched root", func(t *testing.T) {
		// Arrange
		root := t.TempDir()
		buf := &syncBuffer{}
		w := New([]string{root}, NewLogger(buf), Options{})
		require.NoError(t, w.Start())
		t.Cleanup(w.Stop)

		// Act
		target := filepath.Join(root, "file.txt")
		require.NoError(t, os.WriteFile(target, []byte("hello"), 0o644))

		// Assert
		entry := waitForEntry(t, buf, 2*time.Second, func(e Entry) bool {
			return e.Op == OpWrite && e.Path == target
		})
		assert.False(t, entry.IsDir)
	})

	t.Run("detects a file created in a subdirectory created after Start", func(t *testing.T) {
		// Arrange
		root := t.TempDir()
		buf := &syncBuffer{}
		w := New([]string{root}, NewLogger(buf), Options{})
		require.NoError(t, w.Start())
		t.Cleanup(w.Stop)

		// Act
		subdir := filepath.Join(root, "newdir")
		require.NoError(t, os.Mkdir(subdir, 0o750))
		waitForEntry(t, buf, 2*time.Second, func(e Entry) bool {
			return e.Op == OpCreate && e.Path == subdir && e.IsDir
		})
		target := filepath.Join(subdir, "nested.txt")
		require.NoError(t, os.WriteFile(target, []byte("hi"), 0o644))

		// Assert - proves the dynamic recursive watch-add on IN_CREATE works
		entry := waitForEntry(t, buf, 2*time.Second, func(e Entry) bool {
			return e.Op == OpWrite && e.Path == target
		})
		assert.False(t, entry.IsDir)
	})

	t.Run("detects a write after an atomic replace of an existing file", func(t *testing.T) {
		// Arrange
		root := t.TempDir()
		target := filepath.Join(root, "file.txt")
		require.NoError(t, os.WriteFile(target, []byte("v1"), 0o644))
		buf := &syncBuffer{}
		w := New([]string{root}, NewLogger(buf), Options{})
		require.NoError(t, w.Start())
		t.Cleanup(w.Stop)

		// Act - simulate the write-temp-then-rename-over pattern used for atomic saves.
		tmp := target + ".tmp"
		// Linux logs the replace as a rename, darwin as a create.
		matchesTarget := func(e Entry) bool {
			return (e.Op == OpWrite || e.Op == OpCreate || e.Op == OpRename) && e.Path == target
		}

		require.NoError(t, os.WriteFile(tmp, []byte("v2"), 0o644))
		require.NoError(t, os.Rename(tmp, target))
		waitForEntry(t, buf, 2*time.Second, matchesTarget)
		firstCount := countMatching(buf.Entries(t), matchesTarget)

		require.NoError(t, os.WriteFile(tmp, []byte("v3"), 0o644))
		require.NoError(t, os.Rename(tmp, target))

		// Assert - the second replace must also be observed, proving the watch was re-armed.
		require.Eventually(t, func() bool {
			return countMatching(buf.Entries(t), matchesTarget) > firstCount
		}, 2*time.Second, 10*time.Millisecond)
	})

	t.Run("does not log activity inside an excluded directory", func(t *testing.T) {
		// Arrange
		root := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o750))
		buf := &syncBuffer{}
		w := New([]string{root}, NewLogger(buf), Options{})
		require.NoError(t, w.Start())
		t.Cleanup(w.Stop)

		// Act
		ignored := filepath.Join(root, ".git", "HEAD")
		require.NoError(t, os.WriteFile(ignored, []byte("ref"), 0o644))
		// Prove activity in a sibling *is* seen (rules out "nothing was watched at all").
		sibling := filepath.Join(root, "tracked.txt")
		require.NoError(t, os.WriteFile(sibling, []byte("x"), 0o644))
		waitForEntry(t, buf, 2*time.Second, func(e Entry) bool {
			return e.Op == OpWrite && e.Path == sibling
		})

		// Assert
		for _, entry := range buf.Entries(t) {
			assert.NotEqual(t, ignored, entry.Path)
		}
	})

	t.Run("skips a root that does not exist", func(t *testing.T) {
		// Arrange
		buf := &syncBuffer{}
		missing := filepath.Join(t.TempDir(), "does-not-exist")
		w := New([]string{missing}, NewLogger(buf), Options{})

		// Act
		err := w.Start()
		t.Cleanup(w.Stop)

		// Assert
		require.NoError(t, err)
		entry := waitForEntry(t, buf, 2*time.Second, func(e Entry) bool {
			return e.Detail != ""
		})
		assert.Contains(t, entry.Detail, missing)
	})

	t.Run("Stop is idempotent", func(t *testing.T) {
		// Arrange
		root := t.TempDir()
		w := New([]string{root}, NewLogger(&syncBuffer{}), Options{})
		require.NoError(t, w.Start())

		// Act + Assert
		assert.NotPanics(t, func() {
			w.Stop()
			w.Stop()
		})
	})
}
