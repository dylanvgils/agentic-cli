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

func TestParseExtras_single(t *testing.T) {
	// Act
	result := ParseExtras("java")

	// Assert
	assert.Equal(t, []string{"java"}, result)
}

func TestParseExtras_multiple(t *testing.T) {
	// Act
	result := ParseExtras("java,dotnet")

	// Assert
	assert.Equal(t, []string{"java", "dotnet"}, result)
}

func TestParseExtras_whitespace(t *testing.T) {
	// Act
	result := ParseExtras(" java , dotnet ")

	// Assert
	assert.Equal(t, []string{"java", "dotnet"}, result)
}

func TestParseExtras_empty(t *testing.T) {
	// Act
	result := ParseExtras("")

	// Assert
	assert.Empty(t, result)
}

func TestParseExtras_emptySegments(t *testing.T) {
	// Act
	result := ParseExtras(",java,,dotnet,")

	// Assert
	assert.Equal(t, []string{"java", "dotnet"}, result)
}
