package tools

import (
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpencodeEphemeralDirs_returnsExpected(t *testing.T) {
	// Act
	dirs := opencodeEphemeralDirs()

	// Assert
	assert.Equal(t, []sandbox.EphemeralDir{{Path: "/tmp", Size: "1g"}}, dirs)
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
	// Act
	result := renderStage(opencodeStage("base"))

	// Assert - verifies the install script checksum before running it
	assert.Contains(t, result, "OPENCODE_INSTALL_CHECKSUM="+DefaultChecksums.OpencodeInstall)
	assert.Contains(t, result, "curl -fsSL https://opencode.ai/v2/install -o /tmp/opencode_install.sh")
	assert.Contains(t, result, `echo "${OPENCODE_INSTALL_CHECKSUM}  /tmp/opencode_install.sh" | sha256sum -c --quiet -`)
	assert.Contains(t, result, `echo "opencode install script checksum verified OK"`)
	assert.Contains(t, result, "bash /tmp/opencode_install.sh --no-modify-path")
	assert.Contains(t, result, "rm /tmp/opencode_install.sh")
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
