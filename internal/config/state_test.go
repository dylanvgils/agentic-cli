package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadState(t *testing.T) {
	t.Run("no file returns empty state", func(t *testing.T) {
		// Act
		state, err := LoadState(t.TempDir())

		// Assert
		require.NoError(t, err)
		assert.Equal(t, &State{}, state)
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		require.NoError(t, os.WriteFile(StateFile(home), []byte(`not-json`), 0o640))

		// Act
		_, err := LoadState(home)

		// Assert
		require.Error(t, err)
	})
}

func Test_State_Save(t *testing.T) {
	// Arrange
	home := t.TempDir()
	checked := time.Now().Truncate(time.Second)
	state := &State{TrustedDirs: []string{"/example.test"}, LastToolVersionCheck: map[string]time.Time{"claude": checked}}

	// Act
	err := state.Save(home)

	// Assert
	require.NoError(t, err)
	info, err := os.Stat(StateFile(home))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
	reloaded, err := LoadState(home)
	require.NoError(t, err)
	assert.Equal(t, []string{"/example.test"}, reloaded.TrustedDirs)
	assert.True(t, checked.Equal(reloaded.LastToolVersionCheck["claude"]))
}

func Test_State_IsTrusted(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	projects := filepath.Join(base, "projects")
	for _, dir := range []string{"projects/foo", "other", "projects-evil"} {
		require.NoError(t, os.MkdirAll(filepath.Join(base, dir), 0o750))
	}
	cfg := &State{TrustedDirs: []string{projects}}

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
		cfg := &State{TrustedDirs: []string{projects + string(filepath.Separator)}}

		// Act
		result := cfg.IsTrusted(filepath.Join(projects, "foo"))

		// Assert
		assert.True(t, result)
	})

	t.Run("empty config", func(t *testing.T) {
		// Arrange
		cfg := &State{}

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
		cfg := &State{TrustedDirs: []string{link}}

		// Act
		result := cfg.IsTrusted(filepath.Join(base, "other"))

		// Assert
		assert.False(t, result, "an entry the agent can retarget must not trust where it points")
	})
}

func Test_State_Trust(t *testing.T) {
	t.Run("appends and persists", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		dir, err := filepath.EvalSymlinks(t.TempDir())
		require.NoError(t, err)
		cfg := &State{}

		// Act
		err = cfg.Trust(dir, home)

		// Assert
		require.NoError(t, err)
		reloaded, err := LoadState(home)
		require.NoError(t, err)
		assert.Equal(t, []string{dir}, reloaded.TrustedDirs)
	})

	t.Run("dir that can't be resolved is not trusted", func(t *testing.T) {
		// Arrange
		cfg := &State{}

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
		cfg := &State{}

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

func Test_State_ChangedSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".agenticrc.toml")
	mounts := func(m ...string) RCLayer {
		rc := &AgenticRC{}
		rc.Run.ExtraMounts = m
		return RCLayer{Path: path, RC: rc}
	}

	t.Run("unset setting never approved is unchanged", func(t *testing.T) {
		// Arrange
		cfg := &State{}

		// Act
		changed := cfg.ChangedSettings(mounts())

		// Assert
		assert.Empty(t, changed)
	})

	t.Run("new setting is changed", func(t *testing.T) {
		// Arrange
		cfg := &State{}

		// Act
		changed := cfg.ChangedSettings(mounts("~/.example:/x:rw"))

		// Assert
		assert.Equal(t, []GuardedSetting{{"run.extra_mounts", []string{"~/.example:/x:rw"}}}, changed)
	})

	t.Run("approved setting is unchanged", func(t *testing.T) {
		// Arrange
		cfg := &State{}
		require.NoError(t, cfg.ApproveSettings(mounts("~/.example:/x:rw"), t.TempDir()))

		// Act
		changed := cfg.ChangedSettings(mounts("~/.example:/x:rw"))

		// Assert
		assert.Empty(t, changed)
	})

	t.Run("cleared approved setting is changed", func(t *testing.T) {
		// Arrange
		cfg := &State{}
		require.NoError(t, cfg.ApproveSettings(mounts("~/.example:/x:rw"), t.TempDir()))

		// Act
		changed := cfg.ChangedSettings(mounts())

		// Assert
		require.Len(t, changed, 1)
		assert.Equal(t, "run.extra_mounts", changed[0].Key)
	})

	t.Run("config file linked to an approved one needs its own approval", func(t *testing.T) {
		// Arrange
		cfg := &State{}
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

	t.Run("approved file reached through a linked dir is unchanged", func(t *testing.T) {
		// Arrange
		cfg := &State{}
		require.NoError(t, cfg.ApproveSettings(mounts("~/.example:/x:rw"), t.TempDir()))
		link := filepath.Join(t.TempDir(), "link")
		symlinkOrSkip(t, filepath.Dir(path), link)
		layer := mounts("~/.example:/x:rw")
		layer.Path = filepath.Join(link, ".agenticrc.toml")

		// Act
		changed := cfg.ChangedSettings(layer)

		// Assert
		assert.Empty(t, changed)
	})
}

func Test_State_ApproveSettings(t *testing.T) {
	// Arrange
	home := t.TempDir()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	rc := &AgenticRC{Namespace: "work"}
	layer := RCLayer{Path: filepath.Join(dir, ".agenticrc.toml"), RC: rc}
	cfg := &State{}

	// Act
	err = cfg.ApproveSettings(layer, home)

	// Assert
	require.NoError(t, err)
	reloaded, err := LoadState(home)
	require.NoError(t, err)
	approved := reloaded.ApprovedSettings[layer.Path]
	assert.Equal(t, map[string]string{"namespace": GuardedSetting{"namespace", "work"}.hash()}, approved)
}

func TestStateFile(t *testing.T) {
	// Act
	path := StateFile("/example.test/agentic")

	// Assert
	assert.Equal(t, filepath.Join("/example.test/agentic", "state.json"), path)
}
