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

func TestVersionScript_returnsAgenticVersionLang(t *testing.T) {
	// Act
	result := versionScript("node")

	// Assert
	assert.Equal(t, "agentic-version-node", result)
}
