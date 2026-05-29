package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPruneImages(t *testing.T) {
	t.Run("returns reclaimed space", func(t *testing.T) {
		// Arrange
		stubDockerRun(t, func(args ...string) (string, error) {
			return "Deleted Images:\nTotal reclaimed space: 1.23GB\n", nil
		})

		// Act
		result, err := PruneImages()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "1.23GB", result)
	})

	t.Run("zero reclaimed returns empty", func(t *testing.T) {
		// Arrange
		stubDockerRun(t, func(args ...string) (string, error) {
			return "Total reclaimed space: 0B\n", nil
		})

		// Act
		result, err := PruneImages()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "", result)
	})

	t.Run("no reclaimed line returns empty", func(t *testing.T) {
		// Arrange
		stubDockerRun(t, func(args ...string) (string, error) {
			return "nothing useful\n", nil
		})

		// Act
		result, err := PruneImages()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "", result)
	})

	t.Run("docker error returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", fmt.Errorf("docker daemon not running"))

		// Act
		_, err := PruneImages()

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})

	t.Run("passes correct args", func(t *testing.T) {
		// Arrange
		var capturedArgs []string
		stubDockerRun(t, func(args ...string) (string, error) {
			capturedArgs = args
			return "", nil
		})

		// Act
		_, _ = PruneImages()

		// Assert
		assert.Equal(t, []string{"image", "prune", "--force"}, capturedArgs)
	})
}
