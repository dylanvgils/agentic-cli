package run

import (
	"errors"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_checkTrust(t *testing.T) {
	t.Run("already trusted skips prompt", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		dir := t.TempDir()
		cfg := &config.CliConfig{}
		require.NoError(t, cfg.Trust(dir, toolHome))
		p := &fakePrompter{}

		// Act
		err := checkTrust(dir, toolHome, false, p)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, p.trustAsked)
	})

	t.Run("trust flag trusts without prompting", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		dir := t.TempDir()
		p := &fakePrompter{}

		// Act
		err := checkTrust(dir, toolHome, true, p)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, p.trustAsked)
		cfg, err := config.LoadConfig(toolHome)
		require.NoError(t, err)
		assert.Contains(t, cfg.TrustedDirs, dir)
	})

	t.Run("approved prompt trusts dir", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		dir := t.TempDir()
		p := &fakePrompter{}

		// Act
		err := checkTrust(dir, toolHome, false, p)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{dir}, p.trustAsked)
		cfg, err := config.LoadConfig(toolHome)
		require.NoError(t, err)
		assert.Contains(t, cfg.TrustedDirs, dir)
	})

	t.Run("refused prompt returns its error and saves nothing", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		dir := t.TempDir()
		p := &fakePrompter{trustDir: func(string) error { return errors.New("directory not trusted") }}

		// Act
		err := checkTrust(dir, toolHome, false, p)

		// Assert
		require.EqualError(t, err, "directory not trusted")
		cfg, err := config.LoadConfig(toolHome)
		require.NoError(t, err)
		assert.NotContains(t, cfg.TrustedDirs, dir)
	})
}

func Test_checkCredentials(t *testing.T) {
	t.Run("layer without credentials skips prompt", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		layers := []config.RCLayer{{Path: "/example.test/.agenticrc.toml", RC: &config.AgenticRC{}}}
		p := &fakePrompter{}

		// Act
		err := checkCredentials(layers, toolHome, p)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, p.credentialsAsked)
	})

	t.Run("approved prompt saves approval", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		layer := credentialLayer(t, "/example.test/key")
		p := &fakePrompter{}

		// Act
		err := checkCredentials([]config.RCLayer{layer}, toolHome, p)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{layer.Path}, p.credentialsAsked)
		cfg, err := config.LoadConfig(toolHome)
		require.NoError(t, err)
		assert.True(t, cfg.CredentialsApproved(layer.Path, config.CredentialsHash(layer.RC.Run.Proxy.Credentials)))
	})

	t.Run("refused prompt returns its error and saves nothing", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		layer := credentialLayer(t, "/example.test/key")
		p := &fakePrompter{approveCredentials: func(config.RCLayer) error { return errors.New("not approved") }}

		// Act
		err := checkCredentials([]config.RCLayer{layer}, toolHome, p)

		// Assert
		require.EqualError(t, err, "not approved")
		cfg, err := config.LoadConfig(toolHome)
		require.NoError(t, err)
		assert.False(t, cfg.CredentialsApproved(layer.Path, config.CredentialsHash(layer.RC.Run.Proxy.Credentials)))
	})

	t.Run("approved credentials skip prompt", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		layer := credentialLayer(t, "/example.test/key")
		approveCredentials(t, layer, toolHome)
		p := &fakePrompter{}

		// Act
		err := checkCredentials([]config.RCLayer{layer}, toolHome, p)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, p.credentialsAsked)
	})

	t.Run("changed credentials prompt again", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		layer := credentialLayer(t, "/example.test/key")
		approveCredentials(t, layer, toolHome)
		layer.RC.Run.Proxy.Credentials[0].Secret = "/example.test/other"
		p := &fakePrompter{}

		// Act
		err := checkCredentials([]config.RCLayer{layer}, toolHome, p)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{layer.Path}, p.credentialsAsked)
	})
}
