package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanImage(t *testing.T) {
	client := newTestClient()

	t.Run("no containers no images makes no remove calls", func(t *testing.T) {
		// Arrange
		var calls [][]string
		stubDockerRun(t, client, func(args ...string) (string, error) {
			calls = append(calls, args)
			return "", nil
		})

		// Act
		err := client.CleanImage("agentic-claude")

		// Assert
		require.NoError(t, err)
		require.Len(t, calls, 2)
		assert.Equal(t, []string{"ps", "--all", "--quiet", "--filter=label=project=agentic-cli", "--filter=ancestor=agentic-claude"}, calls[0])
		assert.Equal(t, []string{"images", "--quiet", "agentic-claude"}, calls[1])
	})

	// stubClean answers ps with a container and images with an image id, failing the docker subcommand fail.
	stubClean := func(t *testing.T, fail string) func() []dockerCall {
		var calls []dockerCall
		stubDockerRun(t, client, func(args ...string) (string, error) {
			calls = append(calls, dockerCall{args: args})
			if args[0] == fail {
				return "", fmt.Errorf("stub: %s failed", fail)
			}
			switch args[0] {
			case "ps":
				return "c1\nc2", nil
			case "images":
				return "sha256abc\nsha256def", nil
			}
			return "", nil
		})
		return func() []dockerCall { return calls }
	}

	t.Run("removes the image's containers, then the image", func(t *testing.T) {
		// Arrange
		get := stubClean(t, "")

		// Act
		err := client.CleanImage("agentic-claude")

		// Assert
		require.NoError(t, err)
		calls := get()
		require.Len(t, calls, 4)
		assert.Equal(t, []string{"rm", "--force", "--volumes", "c1", "c2"}, calls[1].args)
		assert.Equal(t, []string{"rmi", "--force", "sha256abc", "sha256def"}, calls[3].args)
	})

	for _, fail := range []string{"ps", "rm", "images", "rmi"} {
		t.Run(fail+" error returns error", func(t *testing.T) {
			// Arrange
			stubClean(t, fail)

			// Act
			err := client.CleanImage("agentic-claude")

			// Assert
			assert.ErrorContains(t, err, fail+" failed")
		})
	}
}

func TestCleanBaseImages(t *testing.T) {
	client := newTestClient()

	t.Run("no matching images makes no rmi call", func(t *testing.T) {
		// Arrange
		var calls [][]string
		stubDockerRun(t, client, func(args ...string) (string, error) {
			calls = append(calls, args)
			return "ubuntu\nnginx\nalpine", nil
		})

		// Act
		err := client.CleanBaseImages()

		// Assert
		require.NoError(t, err)
		require.Len(t, calls, 1)
		assert.Equal(t, []string{"images", "--format={{.Repository}}"}, calls[0])
	})

	t.Run("with base images removes all", func(t *testing.T) {
		// Arrange
		callNum := 0
		var rmiArgs []string
		stubDockerRun(t, client, func(args ...string) (string, error) {
			callNum++
			if callNum == 1 {
				return "ubuntu\nagentic-base-java\nagentic-base-dotnet\nnginx", nil
			}
			rmiArgs = args
			return "", nil
		})

		// Act
		err := client.CleanBaseImages()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 2, callNum)
		assert.Equal(t, []string{"rmi", "--force", "agentic-base-java", "agentic-base-dotnet"}, rmiArgs)
	})

	t.Run("images error returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", fmt.Errorf("docker daemon not running"))

		// Act
		err := client.CleanBaseImages()

		// Assert
		require.Error(t, err)
	})

	t.Run("rmi error returns error", func(t *testing.T) {
		// Arrange
		callNum := 0
		stubDockerRun(t, client, func(args ...string) (string, error) {
			callNum++
			if callNum == 1 {
				return "agentic-base-java", nil
			}
			return "", fmt.Errorf("rmi failed")
		})

		// Act
		err := client.CleanBaseImages()

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "rmi failed")
	})
}
