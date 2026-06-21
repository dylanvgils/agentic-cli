package cli

import (
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckTrust(t *testing.T) {
	t.Run("already trusted skips prompt", func(t *testing.T) {
		// Arrange
		toolHomeDir := t.TempDir()
		dir := t.TempDir()
		writeTrustConfig(t, toolHomeDir, []string{dir})
		orig := trustStdin
		trustStdin = strings.NewReader("")
		defer func() { trustStdin = orig }()

		// Act
		err := checkTrust(dir, toolHomeDir, false)

		// Assert
		require.NoError(t, err)
	})

	t.Run("trust flag adds to config", func(t *testing.T) {
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
	})

	t.Run("no tty untrusted returns error", func(t *testing.T) {
		// Arrange
		toolHomeDir := t.TempDir()
		dir := t.TempDir()
		// isTerminal returns false in test environments naturally

		// Act
		err := checkTrust(dir, toolHomeDir, false)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--trust-dir")
	})

	t.Run("tty answers y adds to config", func(t *testing.T) {
		// Arrange
		toolHomeDir := t.TempDir()
		dir := t.TempDir()
		orig := isTerminal
		isTerminal = func() bool { return true }
		defer func() { isTerminal = orig }()
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
	})

	t.Run("tty answers n returns error", func(t *testing.T) {
		// Arrange
		toolHomeDir := t.TempDir()
		dir := t.TempDir()
		orig := isTerminal
		isTerminal = func() bool { return true }
		defer func() { isTerminal = orig }()
		origStdin := trustStdin
		trustStdin = strings.NewReader("n\n")
		defer func() { trustStdin = origStdin }()

		// Act
		err := checkTrust(dir, toolHomeDir, false)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not trusted")
	})

	t.Run("tty empty input returns error", func(t *testing.T) {
		// Arrange
		toolHomeDir := t.TempDir()
		dir := t.TempDir()
		orig := isTerminal
		isTerminal = func() bool { return true }
		defer func() { isTerminal = orig }()
		origStdin := trustStdin
		trustStdin = strings.NewReader("\n")
		defer func() { trustStdin = origStdin }()

		// Act
		err := checkTrust(dir, toolHomeDir, false)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not trusted")
	})
}
