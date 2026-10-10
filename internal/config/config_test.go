package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	t.Run("file not exist returns empty", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()

		// Act
		cfg, err := LoadConfig(dir)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, cfg.TrustedDirs)
		assert.Empty(t, cfg.Registry)
	})

	t.Run("valid file returns parsed", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "agentic.json"),
			[]byte(`{"trusted_dirs":["/home/user/projects"]}`),
			0o640,
		))

		// Act
		cfg, err := LoadConfig(dir)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"/home/user/projects"}, cfg.TrustedDirs)
	})

	t.Run("registry field is loaded", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "agentic.json"),
			[]byte(`{"registry":"myregistry.example.com"}`),
			0o640,
		))

		// Act
		cfg, err := LoadConfig(dir)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "myregistry.example.com", cfg.Registry)
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "agentic.json"),
			[]byte(`not-json`),
			0o640,
		))

		// Act
		_, err := LoadConfig(dir)

		// Assert
		require.Error(t, err)
	})
}

func TestSave_writesFileWithCorrectPerms(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	cfg := &CliConfig{TrustedDirs: []string{"/foo"}}

	// Act
	err := cfg.Save(dir)

	// Assert
	require.NoError(t, err)
	info, err := os.Stat(filepath.Join(dir, "agentic.json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())

	reloaded, err := LoadConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"/foo"}, reloaded.TrustedDirs)
}

func TestSave_registryRoundTrips(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	cfg := &CliConfig{Registry: "myregistry.example.com"}

	// Act
	err := cfg.Save(dir)

	// Assert
	require.NoError(t, err)
	reloaded, err := LoadConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, "myregistry.example.com", reloaded.Registry)
}

func TestSave_lastToolVersionCheckRoundTrips(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	checked := time.Now().Truncate(time.Second)
	cfg := &CliConfig{LastToolVersionCheck: map[string]time.Time{"claude": checked}}

	// Act
	err := cfg.Save(dir)

	// Assert
	require.NoError(t, err)
	reloaded, err := LoadConfig(dir)
	require.NoError(t, err)
	require.Contains(t, reloaded.LastToolVersionCheck, "claude")
	assert.True(t, checked.Equal(reloaded.LastToolVersionCheck["claude"]))
}

func TestIsTrusted(t *testing.T) {
	cfg := &CliConfig{TrustedDirs: []string{"/home/user/projects"}}

	t.Run("exact match", func(t *testing.T) {
		// Act
		result := cfg.IsTrusted("/home/user/projects")

		// Assert
		assert.True(t, result)
	})

	t.Run("parent match", func(t *testing.T) {
		// Act
		result := cfg.IsTrusted("/home/user/projects/foo")

		// Assert
		assert.True(t, result)
	})

	t.Run("no match", func(t *testing.T) {
		// Act
		result := cfg.IsTrusted("/home/user/other")

		// Assert
		assert.False(t, result)
	})

	t.Run("prefix without separator no match", func(t *testing.T) {
		// Act
		result := cfg.IsTrusted("/home/user/projects-evil")

		// Assert
		assert.False(t, result)
	})

	t.Run("empty config", func(t *testing.T) {
		// Arrange
		cfg := &CliConfig{}

		// Act
		result := cfg.IsTrusted("/anything")

		// Assert
		assert.False(t, result)
	})

	t.Run("symlink dir matches", func(t *testing.T) {
		// Arrange: create a real dir and a symlink pointing to it
		real := t.TempDir()
		link := filepath.Join(t.TempDir(), "link")
		if err := os.Symlink(real, link); err != nil {
			t.Skip("cannot create symlink:", err)
		}
		cfg := &CliConfig{TrustedDirs: []string{real}}

		// Act
		result := cfg.IsTrusted(link)

		// Assert
		assert.True(t, result, "symlinked dir should be trusted when its target is trusted")
	})
}

func TestTrust_appendsAndPersists(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	cfg := &CliConfig{}

	// Act
	err := cfg.Trust("/new/dir", dir)

	// Assert
	require.NoError(t, err)
	reloaded, err := LoadConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"/new/dir"}, reloaded.TrustedDirs)
}

func TestCredentialsApproved(t *testing.T) {
	cfg := &CliConfig{ApprovedCredentials: map[string]string{"/example.test/.agenticrc.toml": "abc"}}

	t.Run("matching hash", func(t *testing.T) {
		// Act
		result := cfg.CredentialsApproved("/example.test/.agenticrc.toml", "abc")

		// Assert
		assert.True(t, result)
	})

	t.Run("different hash", func(t *testing.T) {
		// Act
		result := cfg.CredentialsApproved("/example.test/.agenticrc.toml", "def")

		// Assert
		assert.False(t, result)
	})

	t.Run("unknown path", func(t *testing.T) {
		// Act
		result := cfg.CredentialsApproved("/other.test/.agenticrc.toml", "abc")

		// Assert
		assert.False(t, result)
	})

	t.Run("symlinked path matches its target", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		target := filepath.Join(dir, ".agenticrc.toml")
		require.NoError(t, os.WriteFile(target, nil, 0o644))
		link := filepath.Join(dir, "link.toml")
		require.NoError(t, os.Symlink(target, link))
		real, err := filepath.EvalSymlinks(target)
		require.NoError(t, err)
		linked := &CliConfig{ApprovedCredentials: map[string]string{real: "abc"}}

		// Act
		result := linked.CredentialsApproved(link, "abc")

		// Assert
		assert.True(t, result)
	})
}

func TestApproveCredentials(t *testing.T) {
	t.Run("persists the hash", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		cfg := &CliConfig{}

		// Act
		err := cfg.ApproveCredentials("/example.test/.agenticrc.toml", "abc", dir)

		// Assert
		require.NoError(t, err)
		reloaded, err := LoadConfig(dir)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"/example.test/.agenticrc.toml": "abc"}, reloaded.ApprovedCredentials)
	})

	t.Run("replaces an earlier approval", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		cfg := &CliConfig{ApprovedCredentials: map[string]string{"/example.test/.agenticrc.toml": "abc"}}

		// Act
		err := cfg.ApproveCredentials("/example.test/.agenticrc.toml", "def", dir)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "def", cfg.ApprovedCredentials["/example.test/.agenticrc.toml"])
	})
}

func TestPendingCredentials(t *testing.T) {
	creds := []RCCredential{{Preset: "anthropic", Secret: "/example.test/key"}}
	cfg := &CliConfig{ApprovedCredentials: map[string]string{"/approved/.agenticrc.toml": CredentialsHash(creds)}}

	t.Run("approved layer is not pending", func(t *testing.T) {
		// Arrange
		layers := []RCLayer{{Path: "/approved/.agenticrc.toml", RC: &AgenticRC{Run: RCRun{Proxy: RCProxy{Credentials: creds}}}}}

		// Act
		pending := cfg.PendingCredentials(layers)

		// Assert
		assert.Empty(t, pending)
	})

	t.Run("layer without credentials is not pending", func(t *testing.T) {
		// Arrange
		layers := []RCLayer{{Path: "/plain/.agenticrc.toml", RC: &AgenticRC{}}}

		// Act
		pending := cfg.PendingCredentials(layers)

		// Assert
		assert.Empty(t, pending)
	})

	t.Run("new and changed layers are pending in order", func(t *testing.T) {
		// Arrange
		changed := []RCCredential{{Preset: "anthropic", Secret: "/example.test/other"}}
		layers := []RCLayer{
			{Path: "/approved/.agenticrc.toml", RC: &AgenticRC{Run: RCRun{Proxy: RCProxy{Credentials: changed}}}},
			{Path: "/new/.agenticrc.toml", RC: &AgenticRC{Run: RCRun{Proxy: RCProxy{Credentials: creds}}}},
		}

		// Act
		pending := cfg.PendingCredentials(layers)

		// Assert
		require.Len(t, pending, 2)
		assert.Equal(t, "/approved/.agenticrc.toml", pending[0].Path)
		assert.Equal(t, "/new/.agenticrc.toml", pending[1].Path)
	})
}

func TestConfigFile(t *testing.T) {
	// Act
	path := ConfigFile("/example.test/agentic")

	// Assert
	assert.Equal(t, filepath.Join("/example.test/agentic", "agentic.json"), path)
}
