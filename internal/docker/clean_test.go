package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanImage_noContainersNoImages_noRemoveCalls(t *testing.T) {
	// Arrange
	var calls [][]string
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		calls = append(calls, args)
		return "", nil // empty output: no containers, no images
	}
	defer func() { dockerRun = orig }()

	// Act
	err := CleanImage("agentic-claude")

	// Assert
	require.NoError(t, err)
	require.Len(t, calls, 2)
	assert.Equal(t, []string{"ps", "--all", "--quiet", "--filter=label=project=agentic-cli", "--filter=ancestor=agentic-claude"}, calls[0])
	assert.Equal(t, []string{"images", "--quiet", "agentic-claude"}, calls[1])
}

func TestCleanImage_withContainers_removesContainers(t *testing.T) {
	// Arrange
	callNum := 0
	var rmArgs []string
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		callNum++
		switch callNum {
		case 1: // docker ps
			return "c1\nc2", nil
		case 2: // docker rm
			rmArgs = args
			return "", nil
		case 3: // docker images
			return "", nil
		}
		return "", nil
	}
	defer func() { dockerRun = orig }()

	// Act
	err := CleanImage("agentic-claude")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 3, callNum)
	assert.Equal(t, []string{"rm", "--force", "c1", "c2"}, rmArgs)
}

func TestCleanImage_withImages_removesImages(t *testing.T) {
	// Arrange
	callNum := 0
	var rmiArgs []string
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		callNum++
		switch callNum {
		case 1: // docker ps - no containers
			return "", nil
		case 2: // docker images
			return "sha256abc\nsha256def", nil
		case 3: // docker rmi
			rmiArgs = args
			return "", nil
		}
		return "", nil
	}
	defer func() { dockerRun = orig }()

	// Act
	err := CleanImage("agentic-claude")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 3, callNum)
	assert.Equal(t, []string{"rmi", "--force", "sha256abc", "sha256def"}, rmiArgs)
}

func TestCleanImage_psError_returnsError(t *testing.T) {
	// Arrange
	restore := mockDockerRun(t, "", fmt.Errorf("docker daemon not running"))
	defer restore()

	// Act
	err := CleanImage("agentic-claude")

	// Assert
	require.Error(t, err)
}

func TestCleanImage_rmError_returnsError(t *testing.T) {
	// Arrange
	callNum := 0
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		callNum++
		if callNum == 1 {
			return "container1", nil // ps returns a container
		}
		return "", fmt.Errorf("rm failed")
	}
	defer func() { dockerRun = orig }()

	// Act
	err := CleanImage("agentic-claude")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rm failed")
}

func TestCleanImage_imagesError_returnsError(t *testing.T) {
	// Arrange
	callNum := 0
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		callNum++
		if callNum == 1 {
			return "", nil // ps returns nothing
		}
		return "", fmt.Errorf("images failed")
	}
	defer func() { dockerRun = orig }()

	// Act
	err := CleanImage("agentic-claude")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "images failed")
}

func TestCleanImage_rmiError_returnsError(t *testing.T) {
	// Arrange
	callNum := 0
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		callNum++
		switch callNum {
		case 1: // ps - no containers
			return "", nil
		case 2: // images - returns an ID
			return "sha256abc", nil
		case 3: // rmi - fails
			return "", fmt.Errorf("rmi failed")
		}
		return "", nil
	}
	defer func() { dockerRun = orig }()

	// Act
	err := CleanImage("agentic-claude")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rmi failed")
}

func TestCleanBaseImages_noMatchingImages_noRmiCall(t *testing.T) {
	// Arrange
	var calls [][]string
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		calls = append(calls, args)
		return "ubuntu\nnginx\nalpine", nil
	}
	defer func() { dockerRun = orig }()

	// Act
	err := CleanBaseImages()

	// Assert
	require.NoError(t, err)
	require.Len(t, calls, 1)
	assert.Equal(t, []string{"images", "--format={{.Repository}}"}, calls[0])
}

func TestCleanBaseImages_withBaseImages_removesAll(t *testing.T) {
	// Arrange
	callNum := 0
	var rmiArgs []string
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		callNum++
		if callNum == 1 {
			return "ubuntu\nagentic-base-java\nagentic-base-dotnet\nnginx", nil
		}
		rmiArgs = args
		return "", nil
	}
	defer func() { dockerRun = orig }()

	// Act
	err := CleanBaseImages()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, callNum)
	assert.Equal(t, []string{"rmi", "--force", "agentic-base-java", "agentic-base-dotnet"}, rmiArgs)
}

func TestCleanBaseImages_imagesError_returnsError(t *testing.T) {
	// Arrange
	restore := mockDockerRun(t, "", fmt.Errorf("docker daemon not running"))
	defer restore()

	// Act
	err := CleanBaseImages()

	// Assert
	require.Error(t, err)
}

func TestCleanBaseImages_rmiError_returnsError(t *testing.T) {
	// Arrange
	callNum := 0
	orig := dockerRun
	dockerRun = func(args ...string) (string, error) {
		callNum++
		if callNum == 1 {
			return "agentic-base-java", nil
		}
		return "", fmt.Errorf("rmi failed")
	}
	defer func() { dockerRun = orig }()

	// Act
	err := CleanBaseImages()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rmi failed")
}
