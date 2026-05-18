package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- claudeTmpfsMounts ---
func TestClaudeTmpfsMounts_returnsExpected(t *testing.T) {
	// Act
	mounts := claudeTmpfsMounts()

	// Assert
	assert.Equal(t, []string{"/tmp:exec,size=1g"}, mounts)
}

// --- claudeMounts ---
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

// --- setupClaude ---
func TestSetupClaude_createsDataDir(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	err := setupClaude(dir)

	// Assert
	require.NoError(t, err)
	assert.DirExists(t, filepath.Join(dir, "claude", "data"))
}

func TestSetupClaude_createsDefaultJSON(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	err := setupClaude(dir)

	// Assert
	require.NoError(t, err)
	got, err := os.ReadFile(filepath.Join(dir, "claude", ".claude.json"))
	require.NoError(t, err)
	assert.Equal(t, "{}", string(got))
}

func TestSetupClaude_doesNotOverwriteExistingJSON(t *testing.T) {
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
}
