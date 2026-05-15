package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- copilotMounts ---
func TestCopilotMounts_tokenAbsent(t *testing.T) {
	// Arrange
	home := t.TempDir()

	// Act
	mounts := copilotMounts(home)

	// Assert
	assert.Equal(t, []string{
		"$PWD:/workspace",
		"$TOOL_HOME/copilot:$CONTAINER_HOME/.copilot",
	}, mounts)
}

func TestCopilotMounts_tokenPresent(t *testing.T) {
	// Arrange
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".secrets"), 0o700))
	tokenPath := filepath.Join(home, ".secrets", "copilot_token")
	require.NoError(t, os.WriteFile(tokenPath, []byte("token"), 0o600))

	// Act
	mounts := copilotMounts(home)

	// Assert
	assert.Equal(t, []string{
		"$PWD:/workspace",
		"$TOOL_HOME/copilot:$CONTAINER_HOME/.copilot",
		tokenPath + ":/run/secrets/copilot_token:ro",
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
