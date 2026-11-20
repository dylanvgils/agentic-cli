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
		return "Total reclaimed space: 12MB\n", nil
	})

	// Act
	reclaimed, err := client.PruneImages()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "12MB", reclaimed)
	assert.Equal(t, []string{"image", "prune", "--force", "--filter=label=project=agentic-cli"}, capturedArgs)
}

func TestPruneBuildCache(t *testing.T) {
	// Arrange
	client := newTestClient()
	var capturedArgs []string
	stubDockerRun(t, client, func(args ...string) (string, error) {
		capturedArgs = args
		return "Total:\t1.5GB\n", nil
	})

	// Act
	reclaimed, err := client.PruneBuildCache()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "1.5GB", reclaimed)
	assert.Equal(t, []string{"builder", "prune", "--force"}, capturedArgs)
}

func Test_reclaimedSpace(t *testing.T) {
	// Act
	result := reclaimedSpace("")

	// Assert
	assert.Empty(t, result, "no total line gives empty")
}
