package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractVersion(t *testing.T) {
	t.Run("semver", func(t *testing.T) {
		// Act
		result := extractVersion("v24.0.0\n")

		// Assert
		assert.Equal(t, "24.0.0", result)
	})

	t.Run("prefixed output", func(t *testing.T) {
		// Act
		result := extractVersion("go version go1.21.0 linux/amd64\n")

		// Assert
		assert.Equal(t, "1.21.0", result)
	})

	t.Run("docker cli output", func(t *testing.T) {
		// Act
		result := extractVersion("Docker version 29.8.2, build 1a2b3c4\n")

		// Assert
		assert.Equal(t, "29.8.2", result)
	})

	t.Run("windows line ending", func(t *testing.T) {
		// Act
		result := extractVersion("1.0.0\r\n")

		// Assert
		assert.Equal(t, "1.0.0", result)
	})

	t.Run("multi line uses first line", func(t *testing.T) {
		// Act
		result := extractVersion("1.2.3\n4.5.6\n")

		// Assert
		assert.Equal(t, "1.2.3", result)
	})

	t.Run("no version returns empty", func(t *testing.T) {
		// Act
		result := extractVersion("no version here\n")

		// Assert
		assert.Equal(t, "", result)
	})
}

func TestParseVersion_delegatesToExtractVersion(t *testing.T) {
	// Act
	result := ParseVersion("claude v3.7.0")

	// Assert
	assert.Equal(t, "3.7.0", result)
}

func TestRunVersionScript(t *testing.T) {
	client := newTestClient()

	t.Run("returns detected version", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "1.2.3\n", nil)

		// Act
		result := client.runVersionScript("agentic-claude", "agentic-version-claude")

		// Assert
		assert.Equal(t, "1.2.3", result)
	})

	t.Run("docker run error returns empty", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", fmt.Errorf("not found"))

		// Act
		result := client.runVersionScript("agentic-claude", "agentic-version-claude")

		// Assert
		assert.Equal(t, "", result)
	})

	t.Run("runs without network so the tool can't self-update mid-detection", func(t *testing.T) {
		// Arrange
		var capturedArgs []string
		stubDockerRun(t, client, func(args ...string) (string, error) {
			capturedArgs = args
			return "1.2.3\n", nil
		})

		// Act
		client.runVersionScript("agentic-claude", "agentic-version-claude")

		// Assert
		assert.Contains(t, capturedArgs, "--network=none")
	})
}

func TestCollectExtraVersions(t *testing.T) {
	client := newTestClient()

	t.Run("empty extras returns empty map", func(t *testing.T) {
		// Arrange
		calls := 0
		stubDockerRun(t, client, func(_ ...string) (string, error) { calls++; return "", nil })

		// Act
		result := client.collectExtraVersions("agentic-base", nil)

		// Assert
		assert.Empty(t, result)
		assert.Equal(t, 0, calls, "docker should not be called for empty extras")
	})

	t.Run("detects version for each extra", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "21.0.1\n", nil)

		// Act
		result := client.collectExtraVersions("agentic-base", []string{"java", "python"})

		// Assert
		assert.Equal(t, "21.0.1", result["java"])
		assert.Equal(t, "21.0.1", result["python"])
	})

	t.Run("docker run error stores empty string", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", fmt.Errorf("fail"))

		// Act
		result := client.collectExtraVersions("agentic-base", []string{"java"})

		// Assert
		assert.Equal(t, "", result["java"])
	})
}

func TestCollectBaseLabel(t *testing.T) {
	client := newTestClient()

	t.Run("no extras returns empty string", func(t *testing.T) {
		// Arrange
		calls := 0
		stubDockerRun(t, client, func(_ ...string) (string, error) { calls++; return "", nil })

		// Act
		result := client.collectBaseLabel("agentic-base", nil)

		// Assert
		assert.Equal(t, "", result)
		assert.Equal(t, 0, calls, "docker should not be called for empty extras")
	})

	t.Run("with extras returns detected versions", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "21.0.1\n", nil)

		// Act
		result := client.collectBaseLabel("agentic-base", []string{"java"})

		// Assert
		assert.Equal(t, "java@21.0.1", result)
	})

	t.Run("version detection fails returns partial label", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", fmt.Errorf("not found"))

		// Act
		result := client.collectBaseLabel("agentic-base", []string{"java"})

		// Assert
		assert.Equal(t, "java", result)
	})
}
