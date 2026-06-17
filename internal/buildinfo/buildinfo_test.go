package buildinfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsDev(t *testing.T) {
	t.Run("empty string is dev", func(t *testing.T) {
		// Act
		result := IsDev("")

		// Assert
		assert.True(t, result)
	})

	t.Run("dev is dev", func(t *testing.T) {
		// Act
		result := IsDev("dev")

		// Assert
		assert.True(t, result)
	})

	t.Run("released version is not dev", func(t *testing.T) {
		// Act
		result := IsDev("v1.2.3")

		// Assert
		assert.False(t, result)
	})
}

func TestIsDevBuild(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	t.Run("dev build", func(t *testing.T) {
		// Arrange
		Version = "dev"

		// Act
		result := IsDevBuild()

		// Assert
		assert.True(t, result)
	})

	t.Run("released build", func(t *testing.T) {
		// Arrange
		Version = "v1.2.3"

		// Act
		result := IsDevBuild()

		// Assert
		assert.False(t, result)
	})
}
