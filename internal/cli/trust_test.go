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

func Test_checkCredentials(t *testing.T) {
	t.Run("layer without credentials skips prompt", func(t *testing.T) {
		// Arrange
		toolHomeDir := t.TempDir()
		layers := []config.RCLayer{{Path: "/example.test/.agenticrc.toml", RC: &config.AgenticRC{}}}

		// Act
		err := checkCredentials(layers, toolHomeDir)

		// Assert
		require.NoError(t, err)
	})

	t.Run("no tty with new credentials returns error", func(t *testing.T) {
		// Arrange
		toolHomeDir := t.TempDir()
		layer := credentialLayer(t, "/example.test/key")
		stubIsTerminal(t, false)

		// Act
		err := checkCredentials([]config.RCLayer{layer}, toolHomeDir)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "run interactively")
	})

	t.Run("tty answers y saves approval", func(t *testing.T) {
		// Arrange
		toolHomeDir := t.TempDir()
		layer := credentialLayer(t, "/example.test/key")
		stubIsTerminal(t, true)
		stubTrustStdin(t, "y\n")
		logs := stubLogs(t)

		// Act
		err := checkCredentials([]config.RCLayer{layer}, toolHomeDir)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, logs.String(), "preset anthropic, secret /example.test/key")
		cfg, err := config.LoadConfig(toolHomeDir)
		require.NoError(t, err)
		assert.True(t, cfg.CredentialsApproved(layer.Path, config.CredentialsHash(layer.RC.Run.Proxy.Credentials)))
	})

	t.Run("tty answers n returns error", func(t *testing.T) {
		// Arrange
		toolHomeDir := t.TempDir()
		layer := credentialLayer(t, "/example.test/key")
		stubIsTerminal(t, true)
		stubTrustStdin(t, "n\n")
		stubLogs(t)

		// Act
		err := checkCredentials([]config.RCLayer{layer}, toolHomeDir)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not approved")
	})

	t.Run("approved credentials skip prompt", func(t *testing.T) {
		// Arrange
		toolHomeDir := t.TempDir()
		layer := credentialLayer(t, "/example.test/key")
		cfg := &config.CliConfig{}
		require.NoError(t, cfg.ApproveCredentials(layer.Path, config.CredentialsHash(layer.RC.Run.Proxy.Credentials), toolHomeDir))

		// Act
		err := checkCredentials([]config.RCLayer{layer}, toolHomeDir)

		// Assert
		require.NoError(t, err)
	})

	t.Run("changed credentials prompt again", func(t *testing.T) {
		// Arrange
		toolHomeDir := t.TempDir()
		layer := credentialLayer(t, "/example.test/key")
		cfg := &config.CliConfig{}
		require.NoError(t, cfg.ApproveCredentials(layer.Path, config.CredentialsHash(layer.RC.Run.Proxy.Credentials), toolHomeDir))
		layer.RC.Run.Proxy.Credentials[0].Secret = "/example.test/other"

		// Act
		err := checkCredentials([]config.RCLayer{layer}, toolHomeDir)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "new or changed")
	})
}

func Test_describeCredential(t *testing.T) {
	t.Run("preset", func(t *testing.T) {
		// Arrange
		cred := config.RCCredential{Preset: "github", Secret: "~/.secrets/gh"}

		// Act
		desc := describeCredential(cred)

		// Assert
		assert.Equal(t, "preset github, secret ~/.secrets/gh", desc)
	})

	t.Run("custom hosts with env", func(t *testing.T) {
		// Arrange
		cred := config.RCCredential{
			Hosts:  []string{"api.example.test", "*.example.test"},
			Header: "X-Token",
			Env:    []string{"EXAMPLE_TOKEN"},
			Secret: "/example.test/token",
		}

		// Act
		desc := describeCredential(cred)

		// Assert
		assert.Equal(t, "header X-Token on api.example.test, *.example.test, secret /example.test/token, env EXAMPLE_TOKEN", desc)
	})
}

func Test_confirmed(t *testing.T) {
	t.Run("leaves the next line for the next prompt", func(t *testing.T) {
		// Arrange
		stubTrustStdin(t, "y\nn\n")

		// Act
		first := confirmed()
		second := confirmed()

		// Assert
		assert.True(t, first)
		assert.False(t, second)
	})

	t.Run("accepts uppercase and surrounding spaces", func(t *testing.T) {
		// Arrange
		stubTrustStdin(t, "  Y \n")

		// Act
		result := confirmed()

		// Assert
		assert.True(t, result)
	})

	t.Run("eof without newline", func(t *testing.T) {
		// Arrange
		stubTrustStdin(t, "y")

		// Act
		result := confirmed()

		// Assert
		assert.True(t, result)
	})

	t.Run("empty input", func(t *testing.T) {
		// Arrange
		stubTrustStdin(t, "")

		// Act
		result := confirmed()

		// Assert
		assert.False(t, result)
	})
}
