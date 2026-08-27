package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpencodeTmpfsMounts_returnsExpected(t *testing.T) {
	// Act
	mounts := opencodeTmpfsMounts()

	// Assert
	assert.Equal(t, []string{"/tmp:exec,size=1g"}, mounts)
}

func TestOpencodeMounts_returnsExpected(t *testing.T) {
	// Act
	mounts := opencodeMounts()

	// Assert
	assert.Equal(t, []string{
		"$PWD:/workspace",
		"$TOOL_HOME/tools/opencode/data:$CONTAINER_HOME/.opencode",
		"$TOOL_HOME/tools/opencode/share:$CONTAINER_HOME/.local/share/opencode",
		"$TOOL_HOME/tools/opencode/state:$CONTAINER_HOME/.local/state/opencode",
		"$TOOL_HOME/tools/opencode/cache:$CONTAINER_HOME/.cache/opencode",
		"$TOOL_HOME/tools/opencode/config:$CONTAINER_HOME/.config/opencode",
	}, mounts)
}

func TestOpencodeStage(t *testing.T) {
	stage := opencodeStage("base")
	result := renderStage(stage)

	t.Run("from prev stage", func(t *testing.T) {
		// Assert
		assert.Equal(t, "base", stage.From.Image)
		assert.Equal(t, "tool", stage.From.As)
	})

	t.Run("contains container user", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "groupadd -g ${HOST_GID} --non-unique opencode")
		assert.Contains(t, result, "useradd -l -u ${HOST_UID} -g ${HOST_GID} -m -s /bin/bash --non-unique opencode")
	})

	t.Run("contains tool home", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "TOOL_HOME=/home/opencode")
	})

	t.Run("contains version script", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "agentic-version-opencode")
		assert.Contains(t, result, "opencode --version")
	})
}

func TestSetupOpencode_createsSubDirs(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	err := setupOpencode(dir)

	// Assert
	require.NoError(t, err)
	for _, sub := range []string{"data", "share", "state", "cache", "config"} {
		assert.DirExists(t, filepath.Join(dir, ToolsDirName, "opencode", sub))
	}
}

func TestWriteOpencodeInstructions(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ToolsDirName, "opencode", "config"), 0o750))

	// Act
	err := writeOpencodeInstructions(dir, "# Environment\n")

	// Assert
	require.NoError(t, err)
	got, err := os.ReadFile(filepath.Join(dir, ToolsDirName, "opencode", "config", "AGENTS.md"))
	require.NoError(t, err)
	assert.Equal(t, instructionsBeginMarker+"\n# Environment\n"+instructionsEndMarker+"\n", string(got))
}
