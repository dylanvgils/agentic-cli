package fswatch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_isExcludedDir(t *testing.T) {
	t.Run("matches a built-in default", func(t *testing.T) {
		// Act
		result := isExcludedDir(".git", nil)

		// Assert
		assert.True(t, result)
	})

	t.Run("matches an extra name", func(t *testing.T) {
		// Arrange
		extra := []string{"target"}

		// Act
		result := isExcludedDir("target", extra)

		// Assert
		assert.True(t, result)
	})

	t.Run("does not match an unrelated name", func(t *testing.T) {
		// Act
		result := isExcludedDir("src", nil)

		// Assert
		assert.False(t, result)
	})
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
