package tools

import (
	"path/filepath"
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

func TestCopilotStage_containsContainerUser(t *testing.T) {
	// Act
	result := renderStage(copilotStage("base"))

	// Assert
	assert.Contains(t, result, "groupadd -g ${HOST_GID} --non-unique copilot")
	assert.Contains(t, result, "useradd -l -u ${HOST_UID} -g ${HOST_GID} -m -s /bin/bash --non-unique copilot")
}

func TestCopilotStage_containsTokenSetup(t *testing.T) {
	// Act
	result := renderStage(copilotStage("base"))

	// Assert
	assert.Contains(t, result, "copilot_token")
	assert.Contains(t, result, "GITHUB_TOKEN")
}

func TestCopilotStage_containsProjectLabel(t *testing.T) {
	// Act
	result := renderStage(copilotStage("base"))

	// Assert
	assert.Contains(t, result, "project=agentic-cli")
}

func TestCopilotStage_containsVersionScript(t *testing.T) {
	// Act
	result := renderStage(copilotStage("base"))

	// Assert
	assert.Contains(t, result, "agentic-version-copilot")
	assert.Contains(t, result, "copilot --version")
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
