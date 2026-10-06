package docker

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_LatestToolVersion(t *testing.T) {
	t.Run("empty installed version returns not ok", func(t *testing.T) {
		// Arrange
		stubLatestVersion(t, "claude", func() (string, error) { return "1.2.3", nil })

		// Act
		latest, newer, ok := LatestToolVersion("claude", "")

		// Assert
		assert.False(t, ok)
		assert.False(t, newer)
		assert.Empty(t, latest)
	})

	t.Run("no fetcher configured returns not ok", func(t *testing.T) {
		// Arrange
		stubLatestVersion(t, "claude", nil)

		// Act
		latest, newer, ok := LatestToolVersion("claude", "1.2.3")

		// Assert
		assert.False(t, ok)
		assert.False(t, newer)
		assert.Empty(t, latest)
	})

	t.Run("fetch error returns not ok", func(t *testing.T) {
		// Arrange
		stubLatestVersion(t, "claude", func() (string, error) { return "", errors.New("network error") })

		// Act
		latest, newer, ok := LatestToolVersion("claude", "1.2.3")

		// Assert
		assert.False(t, ok)
		assert.False(t, newer)
		assert.Empty(t, latest)
	})

	t.Run("matching versions returns ok and not newer", func(t *testing.T) {
		// Arrange
		stubLatestVersion(t, "claude", func() (string, error) { return "v1.2.3", nil })

		// Act
		latest, newer, ok := LatestToolVersion("claude", "1.2.3")

		// Assert
		assert.True(t, ok)
		assert.False(t, newer)
		assert.Equal(t, "1.2.3", latest)
	})

	t.Run("differing versions returns ok and newer with normalized latest", func(t *testing.T) {
		// Arrange
		stubLatestVersion(t, "claude", func() (string, error) { return "v9.9.9", nil })

		// Act
		latest, newer, ok := LatestToolVersion("claude", "1.2.3")

		// Assert
		assert.True(t, ok)
		assert.True(t, newer)
		assert.Equal(t, "9.9.9", latest)
	})
}
