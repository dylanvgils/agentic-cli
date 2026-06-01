package docker

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStampImageLabels(t *testing.T) {
	var capturedArgs []string
	origStdin := dockerRunStdin
	dockerRunStdin = func(_ io.Reader, args ...string) (string, error) {
		capturedArgs = args
		return "", nil
	}
	t.Cleanup(func() { dockerRunStdin = origStdin })

	t.Run("includes tool label", func(t *testing.T) {
		stubDockerRunFixed(t, "", nil)

		// Act
		stampImageLabels("agentic-claude", "claude", nil, nil)

		// Assert
		assert.Contains(t, capturedArgs, "--label="+LabelTool+"=claude")
	})

	t.Run("includes apt label with packages", func(t *testing.T) {
		stubDockerRunFixed(t, "", nil)

		// Act
		stampImageLabels("agentic-claude", "claude", nil, []string{"make", "gcc"})

		// Assert
		assert.Contains(t, capturedArgs, "--label="+LabelApt+"=make,gcc")
	})

	t.Run("includes base label", func(t *testing.T) {
		stubDockerRunFixed(t, "", nil)

		// Act
		stampImageLabels("agentic-claude", "claude", nil, nil)

		// Assert
		found := false
		for _, a := range capturedArgs {
			if strings.HasPrefix(a, "--label="+LabelBase+"=") {
				found = true
				break
			}
		}
		assert.True(t, found, "expected --%s label in args", LabelBase)
	})

	t.Run("includes tool version label when detected", func(t *testing.T) {
		stubDockerRunFixed(t, "1.2.3\n", nil)

		// Act
		stampImageLabels("agentic-claude", "claude", nil, nil)

		// Assert
		assert.Contains(t, capturedArgs, "--label="+LabelToolVersion+"=1.2.3")
	})

	t.Run("omits tool version label when detection fails", func(t *testing.T) {
		stubDockerRunFixed(t, "", fmt.Errorf("version script not found"))

		// Act
		stampImageLabels("agentic-claude", "claude", nil, nil)

		// Assert
		for _, a := range capturedArgs {
			assert.False(t, strings.HasPrefix(a, "--label="+LabelToolVersion+"="),
				"unexpected %s label in args: %s", LabelToolVersion, a)
		}
	})
}

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
	t.Run("returns detected version", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "1.2.3\n", nil)

		// Act
		result := runVersionScript("agentic-claude", "agentic-version-claude")

		// Assert
		assert.Equal(t, "1.2.3", result)
	})

	t.Run("docker run error returns empty", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", fmt.Errorf("not found"))

		// Act
		result := runVersionScript("agentic-claude", "agentic-version-claude")

		// Assert
		assert.Equal(t, "", result)
	})
}

func TestCollectExtraVersions(t *testing.T) {
	t.Run("empty extras returns empty map", func(t *testing.T) {
		// Arrange
		calls := 0
		stubDockerRun(t, func(_ ...string) (string, error) { calls++; return "", nil })

		// Act
		result := collectExtraVersions("agentic-base", nil)

		// Assert
		assert.Empty(t, result)
		assert.Equal(t, 0, calls, "dockerRun should not be called for empty extras")
	})

	t.Run("detects version for each extra", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "21.0.1\n", nil)

		// Act
		result := collectExtraVersions("agentic-base", []string{"java", "python"})

		// Assert
		assert.Equal(t, "21.0.1", result["java"])
		assert.Equal(t, "21.0.1", result["python"])
	})

	t.Run("docker run error stores empty string", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", fmt.Errorf("fail"))

		// Act
		result := collectExtraVersions("agentic-base", []string{"java"})

		// Assert
		assert.Equal(t, "", result["java"])
	})
}

func TestCollectBaseLabel(t *testing.T) {
	t.Run("no extras returns node label", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "24.0.0\n", nil)

		// Act
		result := collectBaseLabel("agentic-base", nil)

		// Assert
		assert.Equal(t, "node@24.0.0", result)
	})

	t.Run("with extras returns full label", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "21.0.1\n", nil)

		// Act
		result := collectBaseLabel("agentic-base", []string{"java"})

		// Assert
		assert.Equal(t, "node@21.0.1,java@21.0.1", result)
	})

	t.Run("version detection fails returns partial label", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", fmt.Errorf("not found"))

		// Act
		result := collectBaseLabel("agentic-base", []string{"java"})

		// Assert
		assert.Equal(t, "node,java", result)
	})
}
