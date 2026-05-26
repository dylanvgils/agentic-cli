package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	df "github.com/dylanvgils/agentic-cli/internal/dockerfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderStage(stage df.Stage) string {
	return df.File{Stages: []df.Stage{stage}}.Render()
}

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

// --- claudeStage ---

func TestClaudeStage_fromPrevStage(t *testing.T) {
	// Act
	stage := claudeStage("base")

	// Assert
	assert.Equal(t, "base", stage.From.Image)
	assert.Equal(t, "tool", stage.From.As)
}

func TestClaudeStage_containsUserSetup(t *testing.T) {
	// Act
	result := renderStage(claudeStage("base"))

	// Assert
	assert.True(t, strings.Contains(result, "claude"), "expected claude user in stage")
	assert.True(t, strings.Contains(result, "HOST_UID"), "expected HOST_UID arg in stage")
}

func TestClaudeStage_containsEntrypoint(t *testing.T) {
	// Act
	result := renderStage(claudeStage("base"))

	// Assert
	assert.Contains(t, result, "entrypoint.sh")
	assert.Contains(t, result, `exec claude`)
}

func TestClaudeStage_containsToolHome(t *testing.T) {
	// Act
	result := renderStage(claudeStage("base"))

	// Assert
	assert.Contains(t, result, "TOOL_HOME=/home/claude")
}

func TestClaudeStage_containsProjectLabel(t *testing.T) {
	// Act
	result := renderStage(claudeStage("base"))

	// Assert
	assert.Contains(t, result, "project=agentic-cli")
}

func TestClaudeStage_containsVersionScript(t *testing.T) {
	// Act
	result := renderStage(claudeStage("base"))

	// Assert
	assert.Contains(t, result, "agentic-version-claude")
	assert.Contains(t, result, "claude --version")
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
