package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPruneImages_returnsReclaimedSpace(t *testing.T) {
	// Arrange
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		return "Deleted Images:\nTotal reclaimed space: 1.23GB\n", nil
	}
	defer func() { dockerRun = orig }()

	// Act
	result, err := PruneImages()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "1.23GB", result)
}

func TestPruneImages_zeroReclaimed_returnsEmpty(t *testing.T) {
	// Arrange
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		return "Total reclaimed space: 0B\n", nil
	}
	defer func() { dockerRun = orig }()

	// Act
	result, err := PruneImages()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestPruneImages_noReclaimedLine_returnsEmpty(t *testing.T) {
	// Arrange
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		return "nothing useful\n", nil
	}
	defer func() { dockerRun = orig }()

	// Act
	result, err := PruneImages()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestPruneImages_dockerError_returnsError(t *testing.T) {
	// Arrange
	restore := mockDockerRun(t, "", fmt.Errorf("docker daemon not running"))
	defer restore()

	// Act
	_, err := PruneImages()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docker daemon not running")
}

func TestPruneImages_passesCorrectArgs(t *testing.T) {
	// Arrange
	var capturedArgs []string
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	}
	defer func() { dockerRun = orig }()

	// Act
	_, _ = PruneImages()

	// Assert
	assert.Equal(t, []string{"image", "prune", "--force"}, capturedArgs)
}
