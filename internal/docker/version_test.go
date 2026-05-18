package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractVersion_semver(t *testing.T) {
	// Act
	result := extractVersion("v24.0.0\n")

	// Assert
	assert.Equal(t, "24.0.0", result)
}

func TestExtractVersion_prefixedOutput(t *testing.T) {
	// Act
	result := extractVersion("go version go1.21.0 linux/amd64\n")

	// Assert
	assert.Equal(t, "1.21.0", result)
}

func TestExtractVersion_windowsLineEnding(t *testing.T) {
	// Act
	result := extractVersion("1.0.0\r\n")

	// Assert
	assert.Equal(t, "1.0.0", result)
}

func TestExtractVersion_multiLine_usesFirstLine(t *testing.T) {
	// Act
	result := extractVersion("1.2.3\n4.5.6\n")

	// Assert
	assert.Equal(t, "1.2.3", result)
}

func TestExtractVersion_noVersion_returnsEmpty(t *testing.T) {
	// Act
	result := extractVersion("no version here\n")

	// Assert
	assert.Equal(t, "", result)
}

func TestParseVersion_delegatesToExtractVersion(t *testing.T) {
	// Act
	result := ParseVersion("claude v3.7.0")

	// Assert
	assert.Equal(t, "3.7.0", result)
}
