package docker

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPruneImages(t *testing.T) {
	// Arrange
	client := newTestClient()
	var capturedArgs []string
	stubDockerRun(t, client, func(args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	})

	// Act
	err := client.PruneImages()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"image", "prune", "--force", "--filter=label=project=agentic-cli"}, capturedArgs)
}

func TestPruneBuildCache(t *testing.T) {
	// Arrange
	client := newTestClient()
	var capturedArgs []string
	stubDockerRun(t, client, func(args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	})

	// Act
	err := client.PruneBuildCache()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"builder", "prune", "--force", "--filter=label=project=agentic-cli"}, capturedArgs)
}

func TestPruneDangling(t *testing.T) {
	// Arrange
	client := newTestClient()
	var ran []string
	stubDockerRun(t, client, func(args ...string) (string, error) {
		ran = append(ran, args[0])
		if args[0] == "image" {
			return "", errors.New("image prune failed")
		}
		return "", nil
	})

	// Act
	err := client.PruneDangling()

	// Assert - the build cache prune still runs after the image prune fails
	require.ErrorContains(t, err, "image prune failed")
	assert.Equal(t, []string{"image", "builder"}, ran)
}
