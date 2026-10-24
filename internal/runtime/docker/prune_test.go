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
		return "Deleted Images:\ndeleted: sha256:abc\n\nTotal reclaimed space: 12MB\n", nil
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
		return "ID\tRECLAIMABLE\tSIZE\tLAST ACCESSED\nabc\ttrue\t1.5GB\t2 days ago\nTotal:\t1.5GB\n", nil
	})

	// Act
	reclaimed, err := client.PruneBuildCache()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "1.5GB", reclaimed)
	assert.Equal(t, []string{"builder", "prune", "--force"}, capturedArgs)
}

func Test_reclaimedSpace(t *testing.T) {
	t.Run("reads the image prune total", func(t *testing.T) {
		// Act
		result := reclaimedSpace("Total reclaimed space: 0B\n")

		// Assert
		assert.Equal(t, "0B", result)
	})

	t.Run("reads the builder prune total", func(t *testing.T) {
		// Act
		result := reclaimedSpace("Total:\t1.5GB\n")

		// Assert
		assert.Equal(t, "1.5GB", result)
	})

	t.Run("no total line gives empty", func(t *testing.T) {
		// Act
		result := reclaimedSpace("")

		// Assert
		assert.Empty(t, result)
	})
}
