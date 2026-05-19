package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseImage_returnsAgenticBase(t *testing.T) {
	// Act
	result := baseImage()

	// Assert
	assert.Equal(t, "agentic-base", result)
}

func TestBaseLayerImage_singleExtra(t *testing.T) {
	// Act
	result := baseLayerImage("java")

	// Assert
	assert.Equal(t, "agentic-base-java", result)
}

func TestBaseLayerImage_multipleExtras(t *testing.T) {
	// Act
	result := baseLayerImage("java", "go")

	// Assert
	assert.Equal(t, "agentic-base-java-go", result)
}

func TestVersionScript_returnsAgenticVersionLang(t *testing.T) {
	// Act
	result := versionScript("node")

	// Assert
	assert.Equal(t, "agentic-version-node", result)
}
