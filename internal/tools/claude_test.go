package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeTmpfsMounts_returnsExpected(t *testing.T) {
	// Act
	mounts := claudeTmpfsMounts()

	// Assert
	assert.Equal(t, []string{"/tmp:exec,size=1g"}, mounts)
}

func TestClaudeMounts_returnsExpected(t *testing.T) {
	// Act
	mounts := claudeMounts()

	// Assert
	assert.Equal(t, []string{
		"$PWD:/workspace",
		"$TOOL_HOME/tools/claude/data:$CONTAINER_HOME/.claude",
		"$TOOL_HOME/tools/claude/.claude.json:$CONTAINER_HOME/.claude.json",
	}, mounts)
}

func TestClaudeMarketplaceMount_returnsExpected(t *testing.T) {
	// Arrange
	url := "git@example.com:acme/marketplace.git"
	dirName := marketplace.CloneDirName(url)

	// Act
	spec := claudeMarketplaceMount("acme", url)

	// Assert
	assert.Equal(t, "$TOOL_HOME/marketplaces/"+dirName+":$CONTAINER_HOME/marketplaces/acme:ro", spec)
}

func TestSetupClaude(t *testing.T) {
	t.Run("creates data dir", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()

		// Act
		err := setupClaude(dir)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, filepath.Join(dir, ToolsDirName, "claude", "data"))
	})

	t.Run("creates default JSON", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()

		// Act
		err := setupClaude(dir)

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(filepath.Join(dir, ToolsDirName, "claude", ".claude.json"))
		require.NoError(t, err)
		assert.Equal(t, "{}", string(got))
	})

	t.Run("does not overwrite existing JSON", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ToolsDirName, "claude"), 0o750))
		p := filepath.Join(dir, ToolsDirName, "claude", ".claude.json")
		require.NoError(t, os.WriteFile(p, []byte(`{"existing":true}`), 0o640))

		// Act
		err := setupClaude(dir)

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(p)
		require.NoError(t, err)
		assert.Equal(t, `{"existing":true}`, string(got))
	})
}

func TestWriteClaudeInstructions(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ToolsDirName, "claude", "data"), 0o750))

	// Act
	err := writeClaudeInstructions(dir, "# Environment\n")

	// Assert
	require.NoError(t, err)
	got, err := os.ReadFile(filepath.Join(dir, ToolsDirName, "claude", "data", "CLAUDE.md"))
	require.NoError(t, err)
	assert.Equal(t, instructionsBeginMarker+"\n# Environment\n"+instructionsEndMarker+"\n", string(got))
}

func TestClaudeStage(t *testing.T) {
	stage := claudeStage("base")
	result := renderStage(stage)

	t.Run("from prev stage", func(t *testing.T) {
		// Assert
		assert.Equal(t, "base", stage.From.Image)
		assert.Equal(t, "tool", stage.From.As)
	})

	t.Run("contains container user", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "groupadd -g ${HOST_GID} --non-unique claude")
		assert.Contains(t, result, "useradd -l -u ${HOST_UID} -g ${HOST_GID} -m -s /bin/bash --non-unique claude")
	})

	t.Run("contains entrypoint", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "entrypoint.sh")
		assert.Contains(t, result, `exec claude`)
	})

	t.Run("registers only the marketplaces named in AGENTIC_MARKETPLACES before exec", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, `$HOME/marketplaces`)
		assert.Contains(t, result, `AGENTIC_MARKETPLACES`)
		assert.Contains(t, result, `claude plugin marketplace add "$dir" --scope user`)
		assert.NotContains(t, result, `for dir in "$marketplaces_dir"/*/`)
	})

	t.Run("deregisters marketplaces no longer mounted before exec", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, `claude plugin marketplace list --json`)
		assert.Contains(t, result, `claude plugin marketplace remove "$name" --scope user`)
	})

	t.Run("contains tool home", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "TOOL_HOME=/home/claude")
	})

	t.Run("contains version script", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "agentic-version-claude")
		assert.Contains(t, result, "claude --version")
	})
}
