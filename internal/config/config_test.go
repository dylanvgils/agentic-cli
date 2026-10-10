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
	base, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	projects := filepath.Join(base, "projects")
	for _, dir := range []string{"projects/foo", "other", "projects-evil"} {
		require.NoError(t, os.MkdirAll(filepath.Join(base, dir), 0o750))
	}
	cfg := &CliConfig{TrustedDirs: []string{projects}}

	t.Run("exact match", func(t *testing.T) {
		// Act
		result := cfg.IsTrusted(projects)

		// Assert
		assert.True(t, result)
	})

	t.Run("parent match", func(t *testing.T) {
		// Act
		result := cfg.IsTrusted(filepath.Join(projects, "foo"))

		// Assert
		assert.True(t, result)
	})

	t.Run("no match", func(t *testing.T) {
		// Act
		result := cfg.IsTrusted(filepath.Join(base, "other"))

		// Assert
		assert.False(t, result)
	})

	t.Run("prefix without separator no match", func(t *testing.T) {
		// Act
		result := cfg.IsTrusted(filepath.Join(base, "projects-evil"))

		// Assert
		assert.False(t, result)
	})

	t.Run("entry with a trailing separator still matches", func(t *testing.T) {
		// Arrange
		cfg := &CliConfig{TrustedDirs: []string{projects + string(filepath.Separator)}}

		// Act
		result := cfg.IsTrusted(filepath.Join(projects, "foo"))

		// Assert
		assert.True(t, result)
	})

	t.Run("empty config", func(t *testing.T) {
		// Arrange
		cfg := &CliConfig{}

		// Act
		result := cfg.IsTrusted(base)

		// Assert
		assert.False(t, result)
	})

	t.Run("missing dir is not trusted", func(t *testing.T) {
		// Act
		result := cfg.IsTrusted(filepath.Join(projects, "missing"))

		// Assert
		assert.False(t, result)
	})

	t.Run("symlink dir matches", func(t *testing.T) {
		// Arrange
		link := filepath.Join(t.TempDir(), "link")
		symlinkOrSkip(t, projects, link)

		// Act
		result := cfg.IsTrusted(link)

		// Assert
		assert.True(t, result, "symlinked dir should be trusted when its target is trusted")
	})

	t.Run("entry that is a symlink is compared as stored", func(t *testing.T) {
		// Arrange
		link := filepath.Join(t.TempDir(), "link")
		symlinkOrSkip(t, filepath.Join(base, "other"), link)
		cfg := &CliConfig{TrustedDirs: []string{link}}

		// Act
		result := cfg.IsTrusted(filepath.Join(base, "other"))

		// Assert
		assert.False(t, result, "an entry the agent can retarget must not trust where it points")
	})
}

func TestTrust(t *testing.T) {
	t.Run("appends and persists", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		dir, err := filepath.EvalSymlinks(t.TempDir())
		require.NoError(t, err)
		cfg := &CliConfig{}

		// Act
		err = cfg.Trust(dir, home)

		// Assert
		require.NoError(t, err)
		reloaded, err := LoadConfig(home)
		require.NoError(t, err)
		assert.Equal(t, []string{dir}, reloaded.TrustedDirs)
	})

	t.Run("dir that can't be resolved is not trusted", func(t *testing.T) {
		// Arrange
		cfg := &CliConfig{}

		// Act
		err := cfg.Trust(filepath.Join(t.TempDir(), "missing"), t.TempDir())

		// Assert
		assert.Error(t, err)
		assert.Empty(t, cfg.TrustedDirs)
	})

	t.Run("symlinked dir is stored by its real path, so retargeting it needs a new approval", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		approved, err := filepath.EvalSymlinks(t.TempDir())
		require.NoError(t, err)
		link := filepath.Join(t.TempDir(), "sub")
		symlinkOrSkip(t, approved, link)
		cfg := &CliConfig{}

		// Act
		err = cfg.Trust(link, home)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{approved}, cfg.TrustedDirs)
		require.NoError(t, os.Remove(link))
		require.NoError(t, os.Symlink(t.TempDir(), link))
		assert.False(t, cfg.IsTrusted(link), "a retargeted link must not inherit the old approval")
	})
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

	t.Run("path through a symlinked dir matches its real path", func(t *testing.T) {
		// Arrange
		real, err := filepath.EvalSymlinks(t.TempDir())
		require.NoError(t, err)
		link := filepath.Join(t.TempDir(), "link")
		symlinkOrSkip(t, real, link)
		linked := &CliConfig{ApprovedCredentials: map[string]string{filepath.Join(real, ".agenticrc.toml"): "abc"}}

		// Act
		result := linked.CredentialsApproved(filepath.Join(link, ".agenticrc.toml"), "abc")

		// Assert
		assert.True(t, result)
	})

	t.Run("config file linked to an approved one needs its own approval", func(t *testing.T) {
		// Arrange
		approvedDir, err := filepath.EvalSymlinks(t.TempDir())
		require.NoError(t, err)
		approved := filepath.Join(approvedDir, ".agenticrc.toml")
		require.NoError(t, os.WriteFile(approved, nil, 0o644))
		workspace, err := filepath.EvalSymlinks(t.TempDir())
		require.NoError(t, err)
		planted := filepath.Join(workspace, ".agenticrc.toml")
		symlinkOrSkip(t, approved, planted)
		linked := &CliConfig{ApprovedCredentials: map[string]string{approved: "abc"}}

		// Act
		result := linked.CredentialsApproved(planted, "abc")

		// Assert
		assert.False(t, result)
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

func TestChangedSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".agenticrc.toml")
	mounts := func(m ...string) RCLayer {
		rc := &AgenticRC{}
		rc.Run.ExtraMounts = m
		return RCLayer{Path: path, RC: rc}
	}

	t.Run("unset setting never approved is unchanged", func(t *testing.T) {
		// Arrange
		cfg := &CliConfig{}

		// Act
		changed := cfg.ChangedSettings(mounts())

		// Assert
		assert.Empty(t, changed)
	})

	t.Run("new setting is changed", func(t *testing.T) {
		// Arrange
		cfg := &CliConfig{}

		// Act
		changed := cfg.ChangedSettings(mounts("~/.example:/x:rw"))

		// Assert
		assert.Equal(t, []GuardedSetting{{"run.extra_mounts", []string{"~/.example:/x:rw"}}}, changed)
	})

	t.Run("approved setting is unchanged", func(t *testing.T) {
		// Arrange
		cfg := &CliConfig{}
		require.NoError(t, cfg.ApproveSettings(mounts("~/.example:/x:rw"), t.TempDir()))

		// Act
		changed := cfg.ChangedSettings(mounts("~/.example:/x:rw"))

		// Assert
		assert.Empty(t, changed)
	})

	t.Run("cleared approved setting is changed", func(t *testing.T) {
		// Arrange
		cfg := &CliConfig{}
		require.NoError(t, cfg.ApproveSettings(mounts("~/.example:/x:rw"), t.TempDir()))

		// Act
		changed := cfg.ChangedSettings(mounts())

		// Assert
		require.Len(t, changed, 1)
		assert.Equal(t, "run.extra_mounts", changed[0].Key)
	})

	t.Run("config file linked to an approved one needs its own approval", func(t *testing.T) {
		// Arrange
		cfg := &CliConfig{}
		require.NoError(t, cfg.ApproveSettings(mounts("~/.example:/x:rw"), t.TempDir()))
		planted := filepath.Join(t.TempDir(), ".agenticrc.toml")
		symlinkOrSkip(t, path, planted)
		layer := mounts("~/.example:/x:rw")
		layer.Path = planted

		// Act
		changed := cfg.ChangedSettings(layer)

		// Assert
		assert.Len(t, changed, 1)
	})
}

func TestApproveSettings(t *testing.T) {
	// Arrange
	home := t.TempDir()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	rc := &AgenticRC{Namespace: "work"}
	layer := RCLayer{Path: filepath.Join(dir, ".agenticrc.toml"), RC: rc}
	cfg := &CliConfig{}

	// Act
	err = cfg.ApproveSettings(layer, home)

	// Assert
	require.NoError(t, err)
	reloaded, err := LoadConfig(home)
	require.NoError(t, err)
	approved := reloaded.ApprovedSettings[layer.Path]
	assert.Equal(t, map[string]string{"namespace": GuardedSetting{"namespace", "work"}.hash()}, approved)
}
