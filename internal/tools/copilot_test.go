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
