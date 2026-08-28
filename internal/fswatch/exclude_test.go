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
