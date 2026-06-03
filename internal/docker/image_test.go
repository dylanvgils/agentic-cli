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
			"agentic.tool": "claude",
			"agentic.namespace": "agentic",
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
		assert.Equal(t, "agentic", info.Namespace)
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

func Test_extractShortID(t *testing.T) {
	t.Run("returns 12-char short ID", func(t *testing.T) {
		// Act
		id := extractShortID("sha256:a1b2c3d4e5f6abcdef012345678901234567890")

		// Assert
		assert.Equal(t, "a1b2c3d4e5f6", id)
	})

	t.Run("ID shorter than 19 chars returns empty", func(t *testing.T) {
		// Act
		id := extractShortID("sha256:short")

		// Assert
		assert.Empty(t, id)
	})
}

func Test_resolveToolName(t *testing.T) {
	t.Run("tool label takes precedence over parsed name", func(t *testing.T) {
		// Act
		_, tool := resolveToolName("agentic-claude", "copilot", "")

		// Assert
		assert.Equal(t, "copilot", tool)
	})

	t.Run("falls back to parsed tool name when label is empty", func(t *testing.T) {
		// Act
		_, tool := resolveToolName("agentic-claude", "", "")

		// Assert
		assert.Equal(t, "claude", tool)
	})

	t.Run("namespace label takes precedence over parsed name", func(t *testing.T) {
		// Act
		namespace, _ := resolveToolName("agentic-claude", "claude", "myproject")

		// Assert
		assert.Equal(t, "myproject", namespace)
	})

	t.Run("falls back to parsed namespace when label is empty", func(t *testing.T) {
		// Act
		prefix, _ := resolveToolName("myproject-claude", "claude", "")

		// Assert
		assert.Equal(t, "myproject", prefix)
	})

	t.Run("unknown tool with no label returns empty tool", func(t *testing.T) {
		// Act
		_, tool := resolveToolName("agentic-bogus", "", "")

		// Assert
		assert.Empty(t, tool)
	})
}

func Test_listAllRepositories(t *testing.T) {
	t.Run("returns repository names from docker output", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "agentic-claude\nmyproject-copilot\n", nil)

		// Act
		repos, err := listAllRepositories()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"agentic-claude", "myproject-copilot"}, repos)
	})

	t.Run("skips none repositories", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "<none>", nil)

		// Act
		repos, err := listAllRepositories()

		// Assert
		require.NoError(t, err)
		assert.Empty(t, repos)
	})

	t.Run("docker error propagates", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", fmt.Errorf("docker daemon not running"))

		// Act
		_, err := listAllRepositories()

		// Assert
		require.Error(t, err)
	})

	t.Run("passes label filter", func(t *testing.T) {
		// Arrange
		var capturedArgs []string
		stubDockerRun(t, func(args ...string) (string, error) {
			capturedArgs = args
			return "", nil
		})

		// Act
		_, err := listAllRepositories()

		// Assert
		require.NoError(t, err)
		assert.Contains(t, capturedArgs, "--filter=label=project=agentic-cli")
	})

	t.Run("passes extra filters", func(t *testing.T) {
		// Arrange
		var capturedArgs []string
		stubDockerRun(t, func(args ...string) (string, error) {
			capturedArgs = args
			return "", nil
		})

		// Act
		_, err := listAllRepositories(ToolFilter("claude"))

		// Assert
		require.NoError(t, err)
		assert.Contains(t, capturedArgs, "--filter=label=agentic.tool=claude")
	})
}

func TestListAllImages(t *testing.T) {
	t.Run("returns parsed images", func(t *testing.T) {
		// Arrange
		myprojectJSON := `{"Id":"sha256:b2c3d4e5f6a7bcdef012345678901234567890","Config":{"Labels":{"agentic.tool":"claude","agentic.namespace":"myproject"}}}`
		callNum := 0
		stubDockerRun(t, func(args ...string) (string, error) {
			callNum++
			switch callNum {
			case 1: // images --filter label=project=agentic-cli
				return "agentic-claude\nmyproject-claude\n", nil
			case 2: // inspect agentic-claude
				return fullImageJSON, nil
			case 3: // image ls size agentic-claude
				return "512MB", nil
			case 4: // inspect myproject-claude
				return myprojectJSON, nil
			case 5: // image ls size myproject-claude
				return "512MB", nil
			}
			return "", nil
		})

		// Act
		images, err := ListAllImages()

		// Assert
		require.NoError(t, err)
		require.Len(t, images, 2)
		assert.Equal(t, "agentic", images[0].Namespace)
		assert.Equal(t, "claude", images[0].Tool)
		assert.Equal(t, "myproject", images[1].Namespace)
		assert.Equal(t, "claude", images[1].Tool)
	})

	t.Run("mixed tools from label", func(t *testing.T) {
		// Arrange
		callNum := 0
		stubDockerRun(t, func(args ...string) (string, error) {
			callNum++
			switch callNum {
			case 1: // images --filter
				return "agentic-claude\nagentic-copilot\n", nil
			case 2: // inspect claude
				return `{"Id":"sha256:a1b2c3d4e5f6abcdef012345678901234567890","Config":{"Labels":{"agentic.tool":"claude"}}}`, nil
			case 3: // image ls size claude
				return "", nil
			case 4: // inspect copilot
				return `{"Id":"sha256:b2c3d4e5f6a7bcdef012345678901234567890","Config":{"Labels":{"agentic.tool":"copilot"}}}`, nil
			case 5: // image ls size copilot
				return "", nil
			}
			return "", nil
		})

		// Act
		images, err := ListAllImages()

		// Assert
		require.NoError(t, err)
		require.Len(t, images, 2)
		assert.Equal(t, "claude", images[0].Tool)
		assert.Equal(t, "copilot", images[1].Tool)
	})

	t.Run("skips images where inspect returns nil", func(t *testing.T) {
		// Arrange
		callNum := 0
		stubDockerRun(t, func(args ...string) (string, error) {
			callNum++
			switch callNum {
			case 1: // images --filter
				return "agentic-claude\nagentic-copilot\n", nil
			case 2: // inspect claude — image does not exist
				return "", fmt.Errorf("No such image: agentic-claude")
			case 3: // inspect copilot
				return `{"Id":"sha256:b2c3d4e5f6a7bcdef012345678901234567890","Config":{"Labels":{"agentic.tool":"copilot"}}}`, nil
			case 4: // image ls size copilot
				return "512MB", nil
			}
			return "", nil
		})

		// Act
		images, err := ListAllImages()

		// Assert
		require.NoError(t, err)
		require.Len(t, images, 1)
		assert.Equal(t, "copilot", images[0].Tool)
	})
}

func TestBuiltToolsFromImages(t *testing.T) {
	t.Run("empty image list returns empty map", func(t *testing.T) {
		// Act
		result := BuiltToolsFromImages(nil)

		// Assert
		assert.Empty(t, result)
	})

	t.Run("images populate map by tool name", func(t *testing.T) {
		// Arrange
		images := []*ImageInfo{{Tool: "claude"}, {Tool: "copilot"}}

		// Act
		result := BuiltToolsFromImages(images)

		// Assert
		assert.True(t, result["claude"])
		assert.True(t, result["copilot"])
		assert.False(t, result["opencode"])
	})

	t.Run("duplicate tool names collapse to one entry", func(t *testing.T) {
		// Arrange
		images := []*ImageInfo{{Tool: "claude"}, {Tool: "claude"}}

		// Act
		result := BuiltToolsFromImages(images)

		// Assert
		assert.Len(t, result, 1)
		assert.True(t, result["claude"])
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
