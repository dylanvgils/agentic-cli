package tools

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopilotTmpfsMounts_returnsExpected(t *testing.T) {
	// Act
	mounts := copilotTmpfsMounts()

	// Assert
	assert.Equal(t, []string{
		"/tmp:exec,size=1g",
		"$CONTAINER_HOME/.cache:exec,size=1g",
	}, mounts)
}

func TestCopilotMounts(t *testing.T) {
	// Act
	mounts := copilotMounts()

	// Assert
	assert.Equal(t, []string{
		"$PWD:/workspace",
		"$TOOL_HOME/copilot:$CONTAINER_HOME/.copilot",
	}, mounts)
}

func TestCopilotStage(t *testing.T) {
	result := renderStage(copilotStage("base"))

	t.Run("from prev stage", func(t *testing.T) {
		// Arrange
		stage := copilotStage("java")

		// Assert
		assert.Equal(t, "java", stage.From.Image)
		assert.Equal(t, "tool", stage.From.As)
	})

	t.Run("contains container user", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "groupadd -g ${HOST_GID} --non-unique copilot")
		assert.Contains(t, result, "useradd -l -u ${HOST_UID} -g ${HOST_GID} -m -s /bin/bash --non-unique copilot")
	})

	t.Run("contains token setup", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "copilot_token")
		assert.Contains(t, result, "GITHUB_TOKEN")
	})

	t.Run("contains version script", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "agentic-version-copilot")
		assert.Contains(t, result, "copilot --version")
	})
}

func TestSetupCopilot_createsDir(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	err := setupCopilot(dir)

	// Assert
	require.NoError(t, err)
	assert.DirExists(t, filepath.Join(dir, "copilot"))
}
