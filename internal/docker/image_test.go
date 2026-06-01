package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fullImageJSON = `{
	"Id": "sha256:a1b2c3d4e5f6abcdef012345678901234567890",
	"Config": {
		"Labels": {
			"agentic.tool.version": "1.2.3",
			"agentic.base": "node:24",
			"agentic.built": "2026-05-01"
		}
	}
}`

func TestInspectImage(t *testing.T) {
	t.Run("all fields", func(t *testing.T) {
		// Arrange
		callNum := 0
		stubDockerRun(t, func(args ...string) (string, error) {
			callNum++
			switch callNum {
			case 1:
				return fullImageJSON, nil
			case 2:
				return "1.23GB", nil
			}
			return "", nil
		})

		// Act
		info, err := InspectImage("agentic-claude")

		// Assert
		require.NoError(t, err)
		require.NotNil(t, info)
		assert.Equal(t, "agentic-claude", info.Image)
		assert.Equal(t, "agentic", info.Prefix)
		assert.Equal(t, "claude", info.Tool)
		assert.Equal(t, "a1b2c3d4e5f6", info.ID)
		assert.Equal(t, "1.2.3", info.Version)
		assert.Equal(t, "node:24", info.Base)
		assert.Equal(t, "2026-05-01", info.Built)
		assert.Equal(t, "1.23GB", info.Size)
	})

	t.Run("no labels", func(t *testing.T) {
		// Arrange
		callNum := 0
		stubDockerRun(t, func(args ...string) (string, error) {
			callNum++
			if callNum == 1 {
				return `{"Id":"sha256:a1b2c3d4e5f6abcdef012345","Config":{"Labels":{}}}`, nil
			}
			return "", nil
		})

		// Act
		info, err := InspectImage("agentic-claude")

		// Assert
		require.NoError(t, err)
		require.NotNil(t, info)
		assert.Empty(t, info.Version)
		assert.Empty(t, info.Base)
		assert.Empty(t, info.Built)
		assert.Empty(t, info.Size)
	})

	t.Run("docker error returns nil", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", fmt.Errorf("No such image: agentic-missing"))

		// Act
		info, err := InspectImage("agentic-missing")

		// Assert
		require.NoError(t, err)
		assert.Nil(t, info)
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "not json", nil)

		// Act
		info, err := InspectImage("agentic-claude")

		// Assert
		require.Error(t, err)
		assert.Nil(t, info)
	})

	t.Run("short ID no slice", func(t *testing.T) {
		// Arrange - ID shorter than 19 chars means we skip slicing
		callNum := 0
		stubDockerRun(t, func(args ...string) (string, error) {
			callNum++
			if callNum == 1 {
				return `{"Id":"sha256:short","Config":{"Labels":{}}}`, nil
			}
			return "", nil
		})

		// Act
		info, err := InspectImage("agentic-claude")

		// Assert
		require.NoError(t, err)
		require.NotNil(t, info)
		assert.Empty(t, info.ID)
	})

	t.Run("passes image name", func(t *testing.T) {
		// Arrange
		callNum := 0
		var inspectArgs, lsArgs []string
		stubDockerRun(t, func(args ...string) (string, error) {
			callNum++
			switch callNum {
			case 1:
				inspectArgs = args
				return fullImageJSON, nil
			case 2:
				lsArgs = args
				return "1.23GB", nil
			}
			return "", nil
		})

		// Act
		_, err := InspectImage("agentic-opencode")

		// Assert
		require.NoError(t, err)
		assert.Contains(t, inspectArgs, "agentic-opencode")
		assert.Contains(t, lsArgs, "--filter=reference=agentic-opencode")
	})
}

func TestParseImageName(t *testing.T) {
	t.Run("default prefix and known tool", func(t *testing.T) {
		// Act
		prefix, tool, ok := parseImageName("agentic-claude")

		// Assert
		assert.True(t, ok)
		assert.Equal(t, "agentic", prefix)
		assert.Equal(t, "claude", tool)
	})

	t.Run("custom prefix and known tool", func(t *testing.T) {
		// Act
		prefix, tool, ok := parseImageName("myproject-copilot")

		// Assert
		assert.True(t, ok)
		assert.Equal(t, "myproject", prefix)
		assert.Equal(t, "copilot", tool)
	})

	t.Run("multi-segment prefix", func(t *testing.T) {
		// Act
		prefix, tool, ok := parseImageName("my-long-project-opencode")

		// Assert
		assert.True(t, ok)
		assert.Equal(t, "my-long-project", prefix)
		assert.Equal(t, "opencode", tool)
	})

	t.Run("unknown tool returns false", func(t *testing.T) {
		// Act
		_, _, ok := parseImageName("agentic-bogus")

		// Assert
		assert.False(t, ok)
	})

	t.Run("no dash returns false", func(t *testing.T) {
		// Act
		_, _, ok := parseImageName("claude")

		// Assert
		assert.False(t, ok)
	})
}

func TestListAllAgenticImages(t *testing.T) {
	t.Run("returns parsed images", func(t *testing.T) {
		// Arrange
		callNum := 0
		stubDockerRun(t, func(args ...string) (string, error) {
			callNum++
			switch callNum {
			case 1: // images --filter label=project=agentic-cli
				return "agentic-claude\nmyproject-claude\n", nil
			case 2, 4: // inspect
				return fullImageJSON, nil
			case 3, 5: // image ls size
				return "512MB", nil
			}
			return "", nil
		})

		// Act
		images, err := ListAllAgenticImages()

		// Assert
		require.NoError(t, err)
		require.Len(t, images, 2)
		assert.Equal(t, "agentic", images[0].Prefix)
		assert.Equal(t, "claude", images[0].Tool)
		assert.Equal(t, "myproject", images[1].Prefix)
		assert.Equal(t, "claude", images[1].Tool)
	})

	t.Run("skips none images", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "<none>", nil)

		// Act
		images, err := ListAllAgenticImages()

		// Assert
		require.NoError(t, err)
		assert.Empty(t, images)
	})

	t.Run("docker error propagates", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", fmt.Errorf("docker daemon not running"))

		// Act
		_, err := ListAllAgenticImages()

		// Assert
		require.Error(t, err)
	})
}

func TestImageSize(t *testing.T) {
	t.Run("returns size", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "1.23GB", nil)

		// Act
		size := imageSize("agentic-claude")

		// Assert
		assert.Equal(t, "1.23GB", size)
	})

	t.Run("error returns empty", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", fmt.Errorf("docker error"))

		// Act
		size := imageSize("agentic-claude")

		// Assert
		assert.Empty(t, size)
	})
}
