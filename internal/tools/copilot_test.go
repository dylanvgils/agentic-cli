package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/marketplace"
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

func TestCopilotMarketplaceMount_returnsExpected(t *testing.T) {
	// Arrange
	url := "git@example.com:acme/marketplace.git"
	dirName := marketplace.CloneDirName(url)

	// Act
	spec := copilotMarketplaceMount("acme", url)

	// Assert
	assert.Equal(t, "$TOOL_HOME/marketplaces/"+dirName+":$CONTAINER_HOME/marketplaces/acme:ro", spec)
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
		assert.Contains(t, result, `index($0,"(Local: ")`)
		assert.Contains(t, result, `copilot plugin marketplace remove --force "$name"`)
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

func TestWriteCopilotInstructions(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "copilot"), 0o750))

	// Act
	err := writeCopilotInstructions(dir, "# Environment\n")

	// Assert
	require.NoError(t, err)
	got, err := os.ReadFile(filepath.Join(dir, "copilot", "copilot-instructions.md"))
	require.NoError(t, err)
	assert.Equal(t, instructionsBeginMarker+"\n# Environment\n"+instructionsEndMarker+"\n", string(got))
}
