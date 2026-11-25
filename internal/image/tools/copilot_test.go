package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopilotEphemeralDirs_returnsExpected(t *testing.T) {
	// Act
	dirs := copilotEphemeralDirs()

	// Assert
	assert.Equal(t, []sandbox.EphemeralDir{
		{Path: "/tmp", Size: "1g"},
		{Path: "$CONTAINER_HOME/.cache", Size: "1g"},
	}, dirs)
}

func TestCopilotMounts(t *testing.T) {
	// Act
	mounts := copilotMounts()

	// Assert
	assert.Equal(t, []string{
		"$PWD:/workspace",
		"$TOOL_HOME/tools/copilot:$CONTAINER_HOME/.copilot",
	}, mounts)
}

func TestCopilotStage(t *testing.T) {
	result := renderStage(copilotStage("base", ImageInfo{}))

	t.Run("registers only the marketplaces named in AGENTIC_MARKETPLACES before exec", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, `$HOME/marketplaces`)
		assert.Contains(t, result, `AGENTIC_MARKETPLACES`)
		assert.Contains(t, result, `copilot plugin marketplace add "$dir"`)
		assert.NotContains(t, result, `for dir in "$marketplaces_dir"/*/`)
	})

	t.Run("deregisters marketplaces no longer mounted before exec", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, `copilot plugin marketplace list 2>/dev/null | awk`)
		assert.Contains(t, result, `copilot plugin marketplace remove --force "$name"`)
	})

	t.Run("verifies install script checksum before running it", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "COPILOT_INSTALL_CHECKSUM="+DefaultChecksums.CopilotInstall)
		assert.Contains(t, result, "curl -fsSL https://gh.io/copilot-install -o /tmp/copilot_install.sh")
	})
}

func Test_copilotMarketplaces(t *testing.T) {
	// Arrange
	home := t.TempDir()
	marketplaces := filepath.Join(home, "marketplaces")
	require.NoError(t, os.MkdirAll(filepath.Join(marketplaces, "test-kept"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(marketplaces, "test-new"), 0o755))
	log := filepath.Join(home, "copilot.log")
	list := strings.Join([]string{
		"Marketplaces:",
		"  test-kept (Local: " + marketplaces + "/test-kept)",
		"  test-gone (Local: " + marketplaces + "/test-gone)",
		"  test-other (Local: /test-elsewhere/test-other)",
	}, "\n")

	// Act
	runScript(t, "copilot-marketplaces.sh", defaultScriptVars(), "",
		stubCommand(t, t.TempDir(), "copilot"),
		"HOME="+home,
		"TEST_LOG="+log,
		"TEST_LIST="+list,
		"AGENTIC_MARKETPLACES=test-kept,test-new,test-missing",
	)

	// Assert
	assert.Equal(t, "plugin marketplace add "+marketplaces+"/test-new\nplugin marketplace remove --force test-gone\n", readFile(t, log))
}

func TestSetupCopilot_createsDir(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	err := setupCopilot(dir)

	// Assert
	require.NoError(t, err)
	assert.DirExists(t, filepath.Join(dir, ToolsDirName, "copilot"))
}
