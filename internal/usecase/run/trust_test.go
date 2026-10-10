package run

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckContextTrust(t *testing.T) {
	contextLayer := func(dir string) config.RCLayer {
		return config.RCLayer{Path: filepath.Join(dir, ".agenticrc.toml"), RC: &config.AgenticRC{DockerContext: "prod"}}
	}

	t.Run("context set in the dir's own config asks for trust", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		p := &fakePrompter{trustDir: func(string) error { return errors.New("directory not trusted") }}

		// Act
		err := CheckContextTrust(dir, t.TempDir(), []config.RCLayer{contextLayer(dir)}, false, p)

		// Assert
		require.EqualError(t, err, "directory not trusted")
		assert.Equal(t, []string{dir}, p.trustAsked)
	})

	t.Run("context set in a parent dir's config asks for approval, not trust", func(t *testing.T) {
		// Arrange
		parent := t.TempDir()
		dir := filepath.Join(parent, "project")
		p := &fakePrompter{}

		// Act
		err := CheckContextTrust(dir, t.TempDir(), []config.RCLayer{contextLayer(parent)}, false, p)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, p.trustAsked)
		assert.Equal(t, [][]string{{"docker_context"}}, p.settingsAsked)
	})

	t.Run("trusted dir still approves its changed settings", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		p := &fakePrompter{approveSettings: func(config.RCLayer, []config.GuardedSetting) error { return errors.New("settings not approved") }}

		// Act
		err := CheckContextTrust(dir, t.TempDir(), []config.RCLayer{contextLayer(dir)}, true, p)

		// Assert
		require.EqualError(t, err, "settings not approved")
		assert.Equal(t, [][]string{{"docker_context"}}, p.settingsAsked)
	})

	t.Run("config without a context does not ask", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		layer := config.RCLayer{Path: filepath.Join(dir, ".agenticrc.toml"), RC: &config.AgenticRC{}}
		p := &fakePrompter{}

		// Act
		err := CheckContextTrust(dir, t.TempDir(), []config.RCLayer{layer}, false, p)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, p.trustAsked)
	})
}

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
		dir, err := filepath.EvalSymlinks(t.TempDir())
		require.NoError(t, err)
		p := &fakePrompter{}

		// Act
		err = checkTrust(dir, toolHome, true, p)

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
		dir, err := filepath.EvalSymlinks(t.TempDir())
		require.NoError(t, err)
		p := &fakePrompter{}

		// Act
		err = checkTrust(dir, toolHome, false, p)

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

func Test_checkSettings(t *testing.T) {
	mountLayer := func(dir string) config.RCLayer {
		rc := &config.AgenticRC{}
		rc.Run.ExtraMounts = []string{"~/.example:/x:rw"}
		return config.RCLayer{Path: filepath.Join(dir, ".agenticrc.toml"), RC: rc}
	}

	t.Run("changed settings are approved and saved", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		layer := mountLayer(t.TempDir())
		p := &fakePrompter{}

		// Act
		err := checkSettings([]config.RCLayer{layer}, toolHome, p)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, [][]string{{"run.extra_mounts"}}, p.settingsAsked)
		cfg, err := config.LoadConfig(toolHome)
		require.NoError(t, err)
		assert.Empty(t, cfg.ChangedSettings(layer))
	})

	t.Run("refused approval returns its error and saves nothing", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		layer := mountLayer(t.TempDir())
		p := &fakePrompter{approveSettings: func(config.RCLayer, []config.GuardedSetting) error { return errors.New("settings not approved") }}

		// Act
		err := checkSettings([]config.RCLayer{layer}, toolHome, p)

		// Assert
		require.EqualError(t, err, "settings not approved")
		cfg, err := config.LoadConfig(toolHome)
		require.NoError(t, err)
		assert.NotEmpty(t, cfg.ChangedSettings(layer))
	})

	t.Run("every config file is asked about", func(t *testing.T) {
		// Arrange
		parent := t.TempDir()
		layers := []config.RCLayer{mountLayer(parent), mountLayer(filepath.Join(parent, "project"))}
		p := &fakePrompter{}

		// Act
		err := checkSettings(layers, t.TempDir(), p)

		// Assert
		require.NoError(t, err)
		assert.Len(t, p.settingsAsked, 2)
	})
}
