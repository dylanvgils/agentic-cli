package tools

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- copilotTmpfsMounts ---
func TestCopilotTmpfsMounts_returnsExpected(t *testing.T) {
	// Act
	mounts := copilotTmpfsMounts()

	// Assert
	assert.Equal(t, []string{
		"/tmp:exec,size=1g",
		"$CONTAINER_HOME/.cache:exec,size=1g",
	}, mounts)
}

// --- copilotMounts ---
func TestCopilotMounts(t *testing.T) {
	// Act
	mounts := copilotMounts()

	// Assert
	assert.Equal(t, []string{
		"$PWD:/workspace",
		"$TOOL_HOME/copilot:$CONTAINER_HOME/.copilot",
	}, mounts)
}

// --- copilotStage ---

func TestCopilotStage_fromPrevStage(t *testing.T) {
	// Act
	stage := copilotStage("java")

	// Assert
	assert.Equal(t, "java", stage.From.Image)
	assert.Equal(t, "tool", stage.From.As)
}

func TestCopilotStage_containsTokenSetup(t *testing.T) {
	// Act
	result := renderStage(copilotStage("base"))

	// Assert
	assert.True(t, strings.Contains(result, "copilot_token"), "expected token setup in copilot entrypoint")
	assert.True(t, strings.Contains(result, "GITHUB_TOKEN"), "expected GITHUB_TOKEN in copilot entrypoint")
}

func TestCopilotStage_containsProjectLabel(t *testing.T) {
	// Act
	result := renderStage(copilotStage("base"))

	// Assert
	assert.Contains(t, result, "project=agentic-cli")
}

// --- setupCopilot ---
func TestSetupCopilot_createsDir(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	err := setupCopilot(dir)

	// Assert
	require.NoError(t, err)
	assert.DirExists(t, filepath.Join(dir, "copilot"))
}
