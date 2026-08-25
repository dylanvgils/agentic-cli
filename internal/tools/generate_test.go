package tools

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/dockerfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateDockerfile(t *testing.T) {
	t.Run("returns content", func(t *testing.T) {
		// Act
		content, err := GenerateDockerfile("claude", BuildOptions{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "FROM")
		assert.Contains(t, content, "claude")
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act + Assert
		_, err := GenerateDockerfile("unknown", BuildOptions{})
		require.Error(t, err)
	})

	t.Run("custom installs appear in generated content", func(t *testing.T) {
		// Arrange
		opts := BuildOptions{CustomInstalls: []config.RCCustomInstall{{Name: "helm", Run: []string{"curl -fsSL https://get.helm.sh | bash"}}}}

		// Act
		content, err := GenerateDockerfile("claude", opts)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, content, "# helm")
		assert.Contains(t, content, "curl -fsSL https://get.helm.sh | bash")
	})
}

func Test_composeStages(t *testing.T) {
	t.Run("no custom installs matches pre-existing stage order", func(t *testing.T) {
		// Act
		stages, err := composeStages("claude", []string{"java"}, BuildOptions{})

		// Assert
		require.NoError(t, err)
		require.Len(t, stages, 3)
		assert.Equal(t, "base", stages[0].From.As)
		assert.Equal(t, "java", stages[1].From.As)
		assert.Equal(t, "tool", stages[2].From.As)
		assert.Equal(t, "java", stages[2].From.Image)
	})

	t.Run("custom installs stage inserted after extras before tool", func(t *testing.T) {
		// Arrange
		opts := BuildOptions{CustomInstalls: []config.RCCustomInstall{{Name: "helm", Run: []string{"true"}}}}

		// Act
		stages, err := composeStages("claude", []string{"java"}, opts)

		// Assert
		require.NoError(t, err)
		require.Len(t, stages, 4)
		assert.Equal(t, "base", stages[0].From.As)
		assert.Equal(t, "java", stages[1].From.As)
		assert.Equal(t, "custom-installs", stages[2].From.As)
		assert.Equal(t, "java", stages[2].From.Image)
		assert.Equal(t, "tool", stages[3].From.As)
		assert.Equal(t, "custom-installs", stages[3].From.Image)
	})

	t.Run("custom installs stage inserted directly after base when no extras", func(t *testing.T) {
		// Arrange
		opts := BuildOptions{CustomInstalls: []config.RCCustomInstall{{Name: "helm", Run: []string{"true"}}}}

		// Act
		stages, err := composeStages("claude", nil, opts)

		// Assert
		require.NoError(t, err)
		require.Len(t, stages, 3)
		assert.Equal(t, "custom-installs", stages[1].From.As)
		assert.Equal(t, "base", stages[1].From.Image)
	})
}

func TestBuildExtraStages(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		// Act
		stages, prev, err := buildExtraStages([]string{}, "base", nil)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, stages)
		assert.Equal(t, "base", prev)
	})

	t.Run("single", func(t *testing.T) {
		// Act
		stages, prev, err := buildExtraStages([]string{"java"}, "base", nil)

		// Assert
		require.NoError(t, err)
		assert.Len(t, stages, 1)
		assert.Equal(t, "java", stages[0].From.As)
		assert.Equal(t, "java", prev)
	})

	t.Run("multiple", func(t *testing.T) {
		// Act
		stages, prev, err := buildExtraStages([]string{"java", "dotnet"}, "base", nil)

		// Assert
		require.NoError(t, err)
		assert.Len(t, stages, 2)
		assert.Equal(t, "java", stages[0].From.As)
		assert.Equal(t, "dotnet", stages[1].From.As)
		assert.Equal(t, "java", stages[1].From.Image)
		assert.Equal(t, "dotnet", prev)
	})

	t.Run("unknown extra returns error", func(t *testing.T) {
		// Act
		stages, _, err := buildExtraStages([]string{"unknown"}, "base", nil)

		// Assert
		require.Error(t, err)
		assert.Nil(t, stages)
	})
}

func TestResolveToolStage(t *testing.T) {
	t.Run("known tool", func(t *testing.T) {
		// Act
		stage, err := resolveToolStage("claude", "base")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "tool", stage.From.As)
		assert.Equal(t, "base", stage.From.Image)
	})

	t.Run("prepends cache-bust instructions", func(t *testing.T) {
		// Act
		stage, err := resolveToolStage("claude", "base")

		// Assert
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(stage.Instructions), 2)
		assert.Equal(t, dockerfile.Arg{Key: "CACHEBUST", Default: ""}, stage.Instructions[0])
		assert.Equal(t, dockerfile.Run{Command: `: "${CACHEBUST}"`}, stage.Instructions[1])
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act + Assert
		_, err := resolveToolStage("unknown", "base")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown")
	})
}

func TestSortByKnownExtras(t *testing.T) {
	t.Run("sorts into known extras order", func(t *testing.T) {
		// Arrange
		input := []string{"java", "go", "dotnet"}

		// Act
		result := sortByKnownExtras(input)

		// Assert
		assert.Equal(t, []string{"dotnet", "go", "java"}, result)
	})

	t.Run("reverse input produces same result", func(t *testing.T) {
		// Arrange
		forward := sortByKnownExtras([]string{"dotnet", "java"})

		// Act
		reversed := sortByKnownExtras([]string{"java", "dotnet"})

		// Assert
		assert.Equal(t, forward, reversed)
	})

	t.Run("deduplicates", func(t *testing.T) {
		// Act
		result := sortByKnownExtras([]string{"java", "dotnet", "java"})

		// Assert
		assert.Equal(t, []string{"dotnet", "java"}, result)
	})

	t.Run("does not mutate input", func(t *testing.T) {
		// Arrange
		input := []string{"java", "dotnet"}

		// Act
		_ = sortByKnownExtras(input)

		// Assert
		assert.Equal(t, []string{"java", "dotnet"}, input)
	})
}
