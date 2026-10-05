package docker

import (
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
