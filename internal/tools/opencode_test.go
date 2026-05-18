package tools

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- opencodeTmpfsMounts ---
func TestOpencodeTmpfsMounts_returnsExpected(t *testing.T) {
	// Act
	mounts := opencodeTmpfsMounts()

	// Assert
	assert.Equal(t, []string{"/tmp:exec,size=1g"}, mounts)
}

// --- opencodeMounts ---
func TestOpencodeMounts_returnsExpected(t *testing.T) {
	// Act
	mounts := opencodeMounts()

	// Assert
	assert.Equal(t, []string{
		"$PWD:/workspace",
		"$TOOL_HOME/opencode/data:$CONTAINER_HOME/.opencode",
		"$TOOL_HOME/opencode/share:$CONTAINER_HOME/.local/share/opencode",
		"$TOOL_HOME/opencode/state:$CONTAINER_HOME/.local/state/opencode",
		"$TOOL_HOME/opencode/cache:$CONTAINER_HOME/.cache/opencode",
		"$TOOL_HOME/opencode/config:$CONTAINER_HOME/.config/opencode",
	}, mounts)
}

// --- setupOpencode ---
func TestSetupOpencode_createsSubDirs(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	err := setupOpencode(dir)

	// Assert
	require.NoError(t, err)
	for _, sub := range []string{"data", "share", "state", "cache", "config"} {
		assert.DirExists(t, filepath.Join(dir, "opencode", sub))
	}
}
