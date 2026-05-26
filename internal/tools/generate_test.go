package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateDockerfile_returnsContent(t *testing.T) {
	// Act
	content, err := GenerateDockerfile("claude", BuildOptions{})

	// Assert
	require.NoError(t, err)
	assert.Contains(t, content, "FROM")
	assert.Contains(t, content, "claude")
}

func TestGenerateDockerfile_unknownTool_returnsError(t *testing.T) {
	// Act + Assert
	_, err := GenerateDockerfile("unknown", BuildOptions{})
	require.Error(t, err)
}

func TestBuildExtraStages_empty(t *testing.T) {
	// Act
	stages, prev, err := buildExtraStages([]string{}, "base", nil)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, stages)
	assert.Equal(t, "base", prev)
}

func TestBuildExtraStages_single(t *testing.T) {
	// Act
	stages, prev, err := buildExtraStages([]string{"java"}, "base", nil)

	// Assert
	require.NoError(t, err)
	assert.Len(t, stages, 1)
	assert.Equal(t, "java", stages[0].From.As)
	assert.Equal(t, "java", prev)
}

func TestBuildExtraStages_multiple(t *testing.T) {
	// Act
	stages, prev, err := buildExtraStages([]string{"java", "dotnet"}, "base", nil)

	// Assert
	require.NoError(t, err)
	assert.Len(t, stages, 2)
	assert.Equal(t, "java", stages[0].From.As)
	assert.Equal(t, "dotnet", stages[1].From.As)
	assert.Equal(t, "java", stages[1].From.Image)
	assert.Equal(t, "dotnet", prev)
}

func TestBuildExtraStages_unknownExtra_returnsError(t *testing.T) {
	// Act
	stages, _, err := buildExtraStages([]string{"unknown"}, "base", nil)

	// Assert
	require.Error(t, err)
	assert.Nil(t, stages)
}

func TestResolveToolStage_knownTool(t *testing.T) {
	// Act
	stage, err := resolveToolStage("claude", "base")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "tool", stage.From.As)
	assert.Equal(t, "base", stage.From.Image)
}

func TestResolveToolStage_unknownTool_returnsError(t *testing.T) {
	// Act + Assert
	_, err := resolveToolStage("unknown", "base")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

func TestParseExtras_single(t *testing.T) {
	// Act
	result := ParseExtras("java")

	// Assert
	assert.Equal(t, []string{"java"}, result)
}

func TestParseExtras_multiple(t *testing.T) {
	// Act
	result := ParseExtras("dotnet,java")

	// Assert
	assert.Equal(t, []string{"dotnet", "java"}, result)
}

func TestParseExtras_whitespace(t *testing.T) {
	// Act
	result := ParseExtras(" dotnet , java ")

	// Assert
	assert.Equal(t, []string{"dotnet", "java"}, result)
}

func TestParseExtras_empty(t *testing.T) {
	// Act
	result := ParseExtras("")

	// Assert
	assert.Empty(t, result)
}

func TestParseExtras_emptySegments(t *testing.T) {
	// Act
	result := ParseExtras(",dotnet,,java,")

	// Assert
	assert.Equal(t, []string{"dotnet", "java"}, result)
}

func TestSortByKnownExtras_sortsIntoKnownExtrasOrder(t *testing.T) {
	// Arrange
	input := []string{"java", "go", "dotnet"}

	// Act
	result := sortByKnownExtras(input)

	// Assert
	assert.Equal(t, []string{"dotnet", "go", "java"}, result)
}

func TestSortByKnownExtras_reverseInputProducesSameResult(t *testing.T) {
	// Arrange
	forward := sortByKnownExtras([]string{"dotnet", "java"})

	// Act
	reversed := sortByKnownExtras([]string{"java", "dotnet"})

	// Assert
	assert.Equal(t, forward, reversed)
}

func TestSortByKnownExtras_doesNotMutateInput(t *testing.T) {
	// Arrange
	input := []string{"java", "dotnet"}

	// Act
	_ = sortByKnownExtras(input)

	// Assert
	assert.Equal(t, []string{"java", "dotnet"}, input)
}
