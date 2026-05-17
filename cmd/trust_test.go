package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTrustConfig(t *testing.T, toolHome string, dirs []string) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"trusted_dirs": dirs})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(toolHome, "agentic.json"), data, 0o640))
}

func TestCheckTrust_alreadyTrusted_skipsPrompt(t *testing.T) {
	// Arrange
	toolHomeDir := t.TempDir()
	dir := t.TempDir()
	writeTrustConfig(t, toolHomeDir, []string{dir})
	origStdin := trustStdin
	trustStdin = strings.NewReader("")
	defer func() { trustStdin = origStdin }()

	// Act
	err := checkTrust(dir, toolHomeDir, false)

	// Assert
	require.NoError(t, err)
}

func TestCheckTrust_trustFlag_addsToConfig(t *testing.T) {
	// Arrange
	toolHomeDir := t.TempDir()
	dir := t.TempDir()

	// Act
	err := checkTrust(dir, toolHomeDir, true)

	// Assert
	require.NoError(t, err)
	cfg, err := config.LoadConfig(toolHomeDir)
	require.NoError(t, err)
	assert.Contains(t, cfg.TrustedDirs, dir)
}

func TestCheckTrust_noTTY_untrusted_returnsError(t *testing.T) {
	// Arrange
	toolHomeDir := t.TempDir()
	dir := t.TempDir()
	// isTerminal returns false in test environments naturally

	// Act
	err := checkTrust(dir, toolHomeDir, false)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--trust-dir")
}

func TestCheckTrust_tty_answersY_addsToConfig(t *testing.T) {
	// Arrange
	toolHomeDir := t.TempDir()
	dir := t.TempDir()
	origIsTerminal := isTerminal
	isTerminal = func() bool { return true }
	defer func() { isTerminal = origIsTerminal }()
	origStdin := trustStdin
	trustStdin = strings.NewReader("y\n")
	defer func() { trustStdin = origStdin }()

	// Act
	err := checkTrust(dir, toolHomeDir, false)

	// Assert
	require.NoError(t, err)
	cfg, err := config.LoadConfig(toolHomeDir)
	require.NoError(t, err)
	assert.Contains(t, cfg.TrustedDirs, dir)
}

func TestCheckTrust_tty_answersN_returnsError(t *testing.T) {
	// Arrange
	toolHomeDir := t.TempDir()
	dir := t.TempDir()
	origIsTerminal := isTerminal
	isTerminal = func() bool { return true }
	defer func() { isTerminal = origIsTerminal }()
	origStdin := trustStdin
	trustStdin = strings.NewReader("n\n")
	defer func() { trustStdin = origStdin }()

	// Act
	err := checkTrust(dir, toolHomeDir, false)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not trusted")
}

func TestCheckTrust_tty_emptyInput_returnsError(t *testing.T) {
	// Arrange
	toolHomeDir := t.TempDir()
	dir := t.TempDir()
	origIsTerminal := isTerminal
	isTerminal = func() bool { return true }
	defer func() { isTerminal = origIsTerminal }()
	origStdin := trustStdin
	trustStdin = strings.NewReader("\n")
	defer func() { trustStdin = origStdin }()

	// Act
	err := checkTrust(dir, toolHomeDir, false)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not trusted")
}
