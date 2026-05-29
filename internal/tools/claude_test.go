package tools

import (
	"os"
	"path/filepath"
	"testing"

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
		"$TOOL_HOME/claude/data:$CONTAINER_HOME/.claude",
		"$TOOL_HOME/claude/.claude.json:$CONTAINER_HOME/.claude.json",
	}, mounts)
}

func TestSetupClaude(t *testing.T) {
	t.Run("creates data dir", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()

		// Act
		err := setupClaude(dir)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, filepath.Join(dir, "claude", "data"))
	})

	t.Run("creates default JSON", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()

		// Act
		err := setupClaude(dir)

		// Assert
		require.NoError(t, err)
		got, err := os.ReadFile(filepath.Join(dir, "claude", ".claude.json"))
		require.NoError(t, err)
		assert.Equal(t, "{}", string(got))
	})

	t.Run("does not overwrite existing JSON", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "claude"), 0o750))
		p := filepath.Join(dir, "claude", ".claude.json")
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

	t.Run("contains tool home", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "TOOL_HOME=/home/claude")
	})

	t.Run("contains project label", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "project=agentic-cli")
	})

	t.Run("contains version script", func(t *testing.T) {
		// Assert
		assert.Contains(t, result, "agentic-version-claude")
		assert.Contains(t, result, "claude --version")
	})
}
