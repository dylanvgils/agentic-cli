package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanImage(t *testing.T) {
	t.Run("no containers no images makes no remove calls", func(t *testing.T) {
		// Arrange
		var calls [][]string
		stubDockerRun(t, func(args ...string) (string, error) {
			calls = append(calls, args)
			return "", nil
		})

		// Act
		err := CleanImage("agentic-claude")

		// Assert
		require.NoError(t, err)
		require.Len(t, calls, 2)
		assert.Equal(t, []string{"ps", "--all", "--quiet", "--filter=label=project=agentic-cli", "--filter=ancestor=agentic-claude"}, calls[0])
		assert.Equal(t, []string{"images", "--quiet", "agentic-claude"}, calls[1])
	})

	t.Run("with containers removes containers", func(t *testing.T) {
		// Arrange
		callNum := 0
		var rmArgs []string
		stubDockerRun(t, func(args ...string) (string, error) {
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
		})

		// Act
		err := CleanImage("agentic-claude")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 3, callNum)
		assert.Equal(t, []string{"rm", "--force", "c1", "c2"}, rmArgs)
	})

	t.Run("with images removes images", func(t *testing.T) {
		// Arrange
		callNum := 0
		var rmiArgs []string
		stubDockerRun(t, func(args ...string) (string, error) {
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
		})

		// Act
		err := CleanImage("agentic-claude")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 3, callNum)
		assert.Equal(t, []string{"rmi", "--force", "sha256abc", "sha256def"}, rmiArgs)
	})

	t.Run("ps error returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", fmt.Errorf("docker daemon not running"))

		// Act
		err := CleanImage("agentic-claude")

		// Assert
		require.Error(t, err)
	})

	t.Run("rm error returns error", func(t *testing.T) {
		// Arrange
		callNum := 0
		stubDockerRun(t, func(args ...string) (string, error) {
			callNum++
			if callNum == 1 {
				return "container1", nil
			}
			return "", fmt.Errorf("rm failed")
		})

		// Act
		err := CleanImage("agentic-claude")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "rm failed")
	})

	t.Run("images error returns error", func(t *testing.T) {
		// Arrange
		callNum := 0
		stubDockerRun(t, func(args ...string) (string, error) {
			callNum++
			if callNum == 1 {
				return "", nil
			}
			return "", fmt.Errorf("images failed")
		})

		// Act
		err := CleanImage("agentic-claude")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "images failed")
	})

	t.Run("rmi error returns error", func(t *testing.T) {
		// Arrange
		callNum := 0
		stubDockerRun(t, func(args ...string) (string, error) {
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
		})

		// Act
		err := CleanImage("agentic-claude")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "rmi failed")
	})
}

func TestCleanBaseImages(t *testing.T) {
	t.Run("no matching images makes no rmi call", func(t *testing.T) {
		// Arrange
		var calls [][]string
		stubDockerRun(t, func(args ...string) (string, error) {
			calls = append(calls, args)
			return "ubuntu\nnginx\nalpine", nil
		})

		// Act
		err := CleanBaseImages()

		// Assert
		require.NoError(t, err)
		require.Len(t, calls, 1)
		assert.Equal(t, []string{"images", "--format={{.Repository}}"}, calls[0])
	})

	t.Run("with base images removes all", func(t *testing.T) {
		// Arrange
		callNum := 0
		var rmiArgs []string
		stubDockerRun(t, func(args ...string) (string, error) {
			callNum++
			if callNum == 1 {
				return "ubuntu\nagentic-base-java\nagentic-base-dotnet\nnginx", nil
			}
			rmiArgs = args
			return "", nil
		})

		// Act
		err := CleanBaseImages()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 2, callNum)
		assert.Equal(t, []string{"rmi", "--force", "agentic-base-java", "agentic-base-dotnet"}, rmiArgs)
	})

	t.Run("images error returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", fmt.Errorf("docker daemon not running"))

		// Act
		err := CleanBaseImages()

		// Assert
		require.Error(t, err)
	})

	t.Run("rmi error returns error", func(t *testing.T) {
		// Arrange
		callNum := 0
		stubDockerRun(t, func(args ...string) (string, error) {
			callNum++
			if callNum == 1 {
				return "agentic-base-java", nil
			}
			return "", fmt.Errorf("rmi failed")
		})

		// Act
		err := CleanBaseImages()

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "rmi failed")
	})
}
