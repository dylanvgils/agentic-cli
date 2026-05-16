package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseExtras_single(t *testing.T) {
	// Act
	result := parseExtras("java")

	// Assert
	assert.Equal(t, []string{"java"}, result)
}

func TestParseExtras_multiple(t *testing.T) {
	// Act
	result := parseExtras("java,dotnet")

	// Assert
	assert.Equal(t, []string{"java", "dotnet"}, result)
}

func TestParseExtras_whitespace(t *testing.T) {
	// Act
	result := parseExtras(" java , dotnet ")

	// Assert
	assert.Equal(t, []string{"java", "dotnet"}, result)
}

func TestParseExtras_empty(t *testing.T) {
	// Act
	result := parseExtras("")

	// Assert
	assert.Empty(t, result)
}

func TestParseExtras_emptySegments(t *testing.T) {
	// Act
	result := parseExtras(",java,,dotnet,")

	// Assert
	assert.Equal(t, []string{"java", "dotnet"}, result)
}
