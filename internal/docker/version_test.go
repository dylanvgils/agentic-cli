package docker

import (
	"fmt"
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

func TestRunVersionScript_returnsDetectedVersion(t *testing.T) {
	// Arrange
	orig := dockerRun
	dockerRun = func(_ ...string) (string, error) { return "1.2.3\n", nil }
	defer func() { dockerRun = orig }()

	// Act
	result := runVersionScript("agentic-claude", "agentic-version-claude")

	// Assert
	assert.Equal(t, "1.2.3", result)
}

func TestRunVersionScript_dockerRunError_returnsEmpty(t *testing.T) {
	// Arrange
	orig := dockerRun
	dockerRun = func(_ ...string) (string, error) { return "", fmt.Errorf("not found") }
	defer func() { dockerRun = orig }()

	// Act
	result := runVersionScript("agentic-claude", "agentic-version-claude")

	// Assert
	assert.Equal(t, "", result)
}

func TestCollectExtraVersions_emptyExtras_returnsEmptyMap(t *testing.T) {
	// Arrange
	calls := 0
	orig := dockerRun
	dockerRun = func(_ ...string) (string, error) { calls++; return "", nil }
	defer func() { dockerRun = orig }()

	// Act
	result := collectExtraVersions("agentic-base", nil)

	// Assert
	assert.Empty(t, result)
	assert.Equal(t, 0, calls, "dockerRun should not be called for empty extras")
}

func TestCollectExtraVersions_detectsVersionForEachExtra(t *testing.T) {
	// Arrange
	orig := dockerRun
	dockerRun = func(_ ...string) (string, error) { return "21.0.1\n", nil }
	defer func() { dockerRun = orig }()

	// Act
	result := collectExtraVersions("agentic-base", []string{"java", "python"})

	// Assert
	assert.Equal(t, "21.0.1", result["java"])
	assert.Equal(t, "21.0.1", result["python"])
}

func TestCollectExtraVersions_dockerRunError_storesEmptyString(t *testing.T) {
	// Arrange
	orig := dockerRun
	dockerRun = func(_ ...string) (string, error) { return "", fmt.Errorf("fail") }
	defer func() { dockerRun = orig }()

	// Act
	result := collectExtraVersions("agentic-base", []string{"java"})

	// Assert
	assert.Equal(t, "", result["java"])
}

func TestCollectBaseLabel_noExtras_returnsNodeLabel(t *testing.T) {
	// Arrange
	orig := dockerRun
	dockerRun = func(_ ...string) (string, error) { return "24.0.0\n", nil }
	defer func() { dockerRun = orig }()

	// Act
	result := collectBaseLabel("agentic-base", nil)

	// Assert
	assert.Equal(t, "node@24.0.0", result)
}

func TestCollectBaseLabel_withExtras_returnsFullLabel(t *testing.T) {
	// Arrange
	orig := dockerRun
	dockerRun = func(_ ...string) (string, error) { return "21.0.1\n", nil }
	defer func() { dockerRun = orig }()

	// Act
	result := collectBaseLabel("agentic-base", []string{"java"})

	// Assert
	assert.Equal(t, "node@21.0.1,java@21.0.1", result)
}

func TestCollectBaseLabel_versionDetectionFails_returnsPartialLabel(t *testing.T) {
	// Arrange
	orig := dockerRun
	dockerRun = func(_ ...string) (string, error) { return "", fmt.Errorf("not found") }
	defer func() { dockerRun = orig }()

	// Act
	result := collectBaseLabel("agentic-base", []string{"java"})

	// Assert
	assert.Equal(t, "node,java", result)
}
