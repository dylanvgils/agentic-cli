package fswatch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
