package tools

import (
	"path/filepath"
	"strings"
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

// --- opencodeStage ---

func TestOpencodeStage_fromPrevStage(t *testing.T) {
	// Act
	stage := opencodeStage("base")

	// Assert
	assert.Equal(t, "base", stage.From.Image)
	assert.Equal(t, "tool", stage.From.As)
}

func TestOpencodeStage_containsInstallAndUser(t *testing.T) {
	// Act
	result := renderStage(opencodeStage("base"))

	// Assert
	assert.True(t, strings.Contains(result, "opencode"), "expected opencode in stage")
	assert.Contains(t, result, "TOOL_HOME=/home/opencode")
}

func TestOpencodeStage_containsProjectLabel(t *testing.T) {
	// Act
	result := renderStage(opencodeStage("base"))

	// Assert
	assert.Contains(t, result, "project=agentic-cli")
}

func TestOpencodeStage_containsVersionScript(t *testing.T) {
	// Act
	result := renderStage(opencodeStage("base"))

	// Assert
	assert.Contains(t, result, "agentic-version-opencode")
	assert.Contains(t, result, "opencode --version")
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
