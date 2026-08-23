package fswatch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// visited records one walkTree visit call, for assertions in Test_walkTree.
type visited struct {
	path  string
	isDir bool
}

func Test_collapseRoots(t *testing.T) {
	t.Run("dedupes identical roots", func(t *testing.T) {
		// Act
		result := collapseRoots([]string{"/a", "/a"})

		// Assert
		assert.Equal(t, []string{"/a"}, result)
	})

	t.Run("drops a root that is a descendant of another root", func(t *testing.T) {
		// Act
		result := collapseRoots([]string{"/a", "/a/b"})

		// Assert
		assert.Equal(t, []string{"/a"}, result)
	})

	t.Run("keeps non-overlapping roots", func(t *testing.T) {
		// Act
		result := collapseRoots([]string{"/a", "/b"})

		// Assert
		assert.Equal(t, []string{"/a", "/b"}, result)
	})

	t.Run("does not treat a sibling with a shared prefix as a descendant", func(t *testing.T) {
		// Act
		result := collapseRoots([]string{"/a", "/ab"})

		// Assert
		assert.Equal(t, []string{"/a", "/ab"}, result)
	})
}

func Test_walkTree(t *testing.T) {
	logger := NewLogger(&syncBuffer{})

	t.Run("visits root directly when root is not a directory", func(t *testing.T) {
		// Arrange
		root := filepath.Join(t.TempDir(), "file.txt")
		require.NoError(t, os.WriteFile(root, []byte("x"), 0o644))
		var got []visited

		// Act
		walkTree(root, nil, logger, func(path string, isDir bool) {
			got = append(got, visited{path, isDir})
		})

		// Assert
		assert.Equal(t, []visited{{root, false}}, got)
	})

	t.Run("visits root and every entry beneath it", func(t *testing.T) {
		// Arrange
		root := t.TempDir()
		subdir := filepath.Join(root, "subdir")
		require.NoError(t, os.Mkdir(subdir, 0o750))
		nested := filepath.Join(subdir, "nested.txt")
		require.NoError(t, os.WriteFile(nested, []byte("x"), 0o644))
		topLevel := filepath.Join(root, "top.txt")
		require.NoError(t, os.WriteFile(topLevel, []byte("x"), 0o644))
		var got []visited

		// Act
		walkTree(root, nil, logger, func(path string, isDir bool) {
			got = append(got, visited{path, isDir})
		})

		// Assert
		assert.ElementsMatch(t, []visited{
			{root, true},
			{subdir, true},
			{nested, false},
			{topLevel, false},
		}, got)
	})

	t.Run("skips an excluded directory and everything beneath it", func(t *testing.T) {
		// Arrange
		root := t.TempDir()
		excluded := filepath.Join(root, "vendor")
		require.NoError(t, os.Mkdir(excluded, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(excluded, "nested.txt"), []byte("x"), 0o644))
		sibling := filepath.Join(root, "top.txt")
		require.NoError(t, os.WriteFile(sibling, []byte("x"), 0o644))
		var got []visited

		// Act
		walkTree(root, []string{"vendor"}, logger, func(path string, isDir bool) {
			got = append(got, visited{path, isDir})
		})

		// Assert
		assert.ElementsMatch(t, []visited{{root, true}, {sibling, false}}, got)
	})

	t.Run("logs a detail and never visits when root does not exist", func(t *testing.T) {
		// Arrange
		missing := filepath.Join(t.TempDir(), "does-not-exist")
		buf := &syncBuffer{}
		visitedRoot := false

		// Act
		walkTree(missing, nil, NewLogger(buf), func(string, bool) {
			visitedRoot = true
		})

		// Assert
		assert.False(t, visitedRoot)
		entries := buf.Entries(t)
		require.Len(t, entries, 1)
		assert.Contains(t, entries[0].Detail, missing)
	})
}
