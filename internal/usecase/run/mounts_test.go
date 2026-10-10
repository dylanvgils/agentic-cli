package run

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_mountSet_hostPaths(t *testing.T) {
	// Arrange
	home := t.TempDir()
	t.Setenv("HOME", home)
	mounts := mountSet{
		volumes:       []string{"agentic-cache:/cache"},
		secrets:       []string{"token:~/token", "other:$HOME/other:/run/other"},
		toolHome:      t.TempDir(),
		containerHome: "/home/agent",
	}

	// Act
	paths := mounts.hostPaths()

	// Assert
	assert.Equal(t, []string{filepath.Join(home, "token"), filepath.Join(home, "other")}, paths)
}

func Test_newMountSet(t *testing.T) {
	// Arrange
	ws := chdirSymlinkWorkspace(t)
	req := buildRequest{
		in:            Input{ToolHome: t.TempDir(), Volumes: []string{"$PWD/link:/data"}, Secrets: []string{"token:$PWD/link/token"}},
		toolConfig:    tools.Configs["claude"],
		rc:            &config.AgenticRC{},
		containerHome: "/home/agent",
	}

	// Act
	mounts, err := newMountSet(req, nil)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, mounts.volumes, ws.real+":/data:ro")
	assert.Equal(t, []string{"token:" + filepath.Join(ws.real, "token")}, mounts.secrets)
}

func Test_mountSet_resolveMounts(t *testing.T) {
	ws := chdirSymlinkWorkspace(t)
	toolHome := t.TempDir()

	t.Run("volumes and secrets are pinned", func(t *testing.T) {
		// Arrange
		mounts := mountSet{volumes: []string{"$PWD/link:/data"}, secrets: []string{"token:$PWD/link/token"}, toolHome: toolHome, containerHome: "/home/agent"}

		// Act
		resolved, err := mounts.resolveMounts()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{ws.real + ":/data"}, resolved.volumes)
		assert.Equal(t, []string{"token:" + filepath.Join(ws.real, "token")}, resolved.secrets)
	})

	t.Run("escaping secret is refused", func(t *testing.T) {
		// Arrange
		mounts := mountSet{secrets: []string{"key:$PWD/escape/id_ed25519"}, toolHome: toolHome, containerHome: "/home/agent"}

		// Act
		_, err := mounts.resolveMounts()

		// Assert
		assert.ErrorContains(t, err, "leads through a workspace symlink")
	})

	t.Run("mount exposing agentic.json is refused", func(t *testing.T) {
		// Arrange
		mounts := mountSet{volumes: []string{"$TOOL_HOME:/agentic"}, toolHome: toolHome, containerHome: "/home/agent"}

		// Act
		_, err := mounts.resolveMounts()

		// Assert
		assert.ErrorContains(t, err, "would expose")
	})
}

func Test_mountSet_checkConfigNotMounted(t *testing.T) {
	toolHome := t.TempDir()

	t.Run("mounts beside agentic.json are accepted", func(t *testing.T) {
		// Arrange
		mounts := mountSet{volumes: []string{"$TOOL_HOME/tools/claude/data:$CONTAINER_HOME/.claude"}, toolHome: toolHome, containerHome: "/home/agent"}

		// Act
		err := mounts.checkConfigNotMounted()

		// Assert
		assert.NoError(t, err)
	})

	t.Run("mount containing agentic.json is refused", func(t *testing.T) {
		// Arrange
		mounts := mountSet{volumes: []string{"$TOOL_HOME:/agentic"}, toolHome: toolHome, containerHome: "/home/agent"}

		// Act
		err := mounts.checkConfigNotMounted()

		// Assert
		assert.EqualError(t, err, "mount "+toolHome+" would expose "+config.ConfigFile(toolHome)+" to the tool container; mount a narrower path")
	})
}

func Test_mountSet_pinVolume(t *testing.T) {
	// Arrange
	ws := chdirSymlinkWorkspace(t)
	require.NoError(t, os.Symlink(ws.real, filepath.Join(ws.dir, "agentic-cache")))
	mounts := mountSet{toolHome: t.TempDir(), containerHome: "/home/agent"}

	// Act
	spec, err := mounts.pinVolume("agentic-cache:/cache")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "agentic-cache:/cache", spec, "a named volume must not turn into a bind mount of a same-named workspace link")
}

func Test_mountSet_pinSecret(t *testing.T) {
	// Arrange
	ws := chdirSymlinkWorkspace(t)
	mounts := mountSet{toolHome: t.TempDir(), containerHome: "/home/agent"}

	// Act
	spec, err := mounts.pinSecret("token:$PWD/link/token")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "token:"+filepath.Join(ws.real, "token"), spec)
}

func Test_mountSet_pinHost(t *testing.T) {
	ws := chdirSymlinkWorkspace(t)
	mounts := mountSet{toolHome: t.TempDir(), containerHome: "/home/agent"}

	t.Run("pinned host keeps the container side and options", func(t *testing.T) {
		// Act
		spec, err := mounts.pinHost("$PWD/link:/data:ro")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, ws.real+":/data:ro", spec)
	})

	t.Run("spec without a workspace link stays as written", func(t *testing.T) {
		// Act
		spec, err := mounts.pinHost("$PWD:/workspace")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "$PWD:/workspace", spec)
	})

	t.Run("real path that would expand again is refused", func(t *testing.T) {
		// Arrange
		require.NoError(t, os.Mkdir(filepath.Join(ws.dir, "x~"), 0o700))
		require.NoError(t, os.Symlink("x~", filepath.Join(ws.dir, "tilde")))

		// Act
		_, err := mounts.pinHost("$PWD/tilde:/data")

		// Assert
		assert.ErrorContains(t, err, "rename it")
	})
}

func Test_within(t *testing.T) {
	t.Run("sibling with a shared prefix is outside", func(t *testing.T) {
		// Arrange
		root := filepath.Join(t.TempDir(), "data")

		// Act
		result := within(root+"-other", root)

		// Assert
		assert.False(t, result)
	})

	t.Run("root itself is within", func(t *testing.T) {
		// Arrange
		root := t.TempDir()

		// Act
		result := within(root, root)

		// Assert
		assert.True(t, result)
	})

	t.Run("case-insensitive filesystems ignore case", func(t *testing.T) {
		// Arrange
		stubCaseInsensitivePaths(t, true)
		root := filepath.Join(t.TempDir(), "Secrets")

		// Act
		result := within(filepath.Join(root, "token"), strings.ToUpper(root))

		// Assert
		assert.True(t, result)
	})

	t.Run("case-sensitive filesystems compare case", func(t *testing.T) {
		// Arrange
		stubCaseInsensitivePaths(t, false)
		root := filepath.Join(t.TempDir(), "secrets")

		// Act
		result := within(filepath.Join(root, "token"), strings.ToUpper(root))

		// Assert
		assert.False(t, result)
	})

	t.Run("missing path under a symlinked parent resolves through it", func(t *testing.T) {
		// Arrange
		if runtime.GOOS == "windows" {
			t.Skip("symlinks need privileges on windows")
		}
		real := t.TempDir()
		link := filepath.Join(t.TempDir(), "link")
		require.NoError(t, os.Symlink(real, link))

		// Act
		result := within(filepath.Join(link, "missing", "file"), real)

		// Assert
		assert.True(t, result)
	})
}

func Test_realWorkspacePath(t *testing.T) {
	ws := chdirSymlinkWorkspace(t)

	t.Run("path without a link returns empty", func(t *testing.T) {
		// Act
		real, err := realWorkspacePath(filepath.Join(ws.dir, "real", "missing"), ws.dir)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, real)
	})

	t.Run("path below a link inside cwd returns its real path", func(t *testing.T) {
		// Act
		real, err := realWorkspacePath(filepath.Join(ws.dir, "link", "sub"), ws.dir)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(ws.real, "sub"), real)
	})

	t.Run("link out of cwd is refused", func(t *testing.T) {
		// Act
		_, err := realWorkspacePath(filepath.Join(ws.dir, "escape"), ws.dir)

		// Assert
		assert.ErrorContains(t, err, "leads through a workspace symlink to "+ws.outside)
	})

	t.Run("dangling workspace link is refused", func(t *testing.T) {
		// Arrange
		require.NoError(t, os.Symlink(filepath.Join(ws.outside, "missing", "deep"), filepath.Join(ws.dir, "dangling")))

		// Act
		_, err := realWorkspacePath(filepath.Join(ws.dir, "dangling"), ws.dir)

		// Assert
		assert.ErrorContains(t, err, "whose target is missing")
	})

	t.Run("path outside cwd returns empty", func(t *testing.T) {
		// Act
		real, err := realWorkspacePath(ws.outside, ws.dir)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, real)
	})
}

func Test_workspaceRel(t *testing.T) {
	cwd := t.TempDir()

	t.Run("path inside cwd is relative to it", func(t *testing.T) {
		// Act
		rel, ok := workspaceRel(filepath.Join(cwd, "a", "b"), cwd)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, filepath.Join("a", "b"), rel)
	})

	t.Run("path through an alias of cwd is inside", func(t *testing.T) {
		// Arrange
		if runtime.GOOS == "windows" {
			t.Skip("symlinks need privileges on windows")
		}
		alias := filepath.Join(t.TempDir(), "alias")
		require.NoError(t, os.Symlink(cwd, alias))

		// Act
		rel, ok := workspaceRel(filepath.Join(alias, "a"), cwd)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, "a", rel)
	})

	t.Run("case-insensitive filesystems ignore case", func(t *testing.T) {
		// Arrange
		stubCaseInsensitivePaths(t, true)

		// Act
		rel, ok := workspaceRel(filepath.Join(strings.ToUpper(cwd), "a"), cwd)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, "a", rel)
	})

	t.Run("link below cwd stays in rel", func(t *testing.T) {
		// Arrange
		if runtime.GOOS == "windows" {
			t.Skip("symlinks need privileges on windows")
		}
		require.NoError(t, os.Symlink(".", filepath.Join(cwd, "self")))

		// Act
		rel, ok := workspaceRel(filepath.Join(cwd, "self", "a"), cwd)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, filepath.Join("self", "a"), rel)
	})

	t.Run("path written via a symlinked cwd's real path is inside", func(t *testing.T) {
		// Arrange
		if runtime.GOOS == "windows" {
			t.Skip("symlinks need privileges on windows")
		}
		linkedCwd := filepath.Join(t.TempDir(), "cwd")
		require.NoError(t, os.Symlink(cwd, linkedCwd))
		realCwd, err := filepath.EvalSymlinks(cwd)
		require.NoError(t, err)

		// Act
		rel, ok := workspaceRel(filepath.Join(realCwd, "a"), linkedCwd)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, "a", rel)
	})
}

func Test_isInside(t *testing.T) {
	t.Run("parent references are outside", func(t *testing.T) {
		for _, rel := range []string{"..", filepath.Join("..", "x")} {
			// Act
			inside := isInside(rel)

			// Assert
			assert.False(t, inside, rel)
		}
	})

	t.Run("name starting with dots is inside", func(t *testing.T) {
		// Act
		inside := isInside("..foo")

		// Assert
		assert.True(t, inside)
	})
}

func Test_resolveExisting(t *testing.T) {
	// Arrange
	ws := chdirSymlinkWorkspace(t)

	// Act
	real := resolveExisting(filepath.Join(ws.dir, "escape", "missing"))

	// Assert
	assert.Equal(t, filepath.Join(ws.outside, "missing"), real)
}

func Test_pinAll(t *testing.T) {
	t.Run("pins every spec in order", func(t *testing.T) {
		// Arrange
		pin := func(spec string) (string, error) { return spec + ":pinned", nil }

		// Act
		pinned, err := pinAll([]string{"a", "b"}, pin)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"a:pinned", "b:pinned"}, pinned)
	})

	t.Run("stops at the first error", func(t *testing.T) {
		// Arrange
		var seen []string
		pin := func(spec string) (string, error) {
			seen = append(seen, spec)
			return "", errors.New("refused")
		}

		// Act
		_, err := pinAll([]string{"a", "b"}, pin)

		// Assert
		assert.EqualError(t, err, "refused")
		assert.Equal(t, []string{"a"}, seen)
	})
}

func Test_danglingLink(t *testing.T) {
	ws := chdirSymlinkWorkspace(t)

	t.Run("link with a missing target is found", func(t *testing.T) {
		// Arrange
		link := filepath.Join(ws.dir, "gone")
		require.NoError(t, os.Symlink(filepath.Join(ws.dir, "missing"), link))

		// Act
		found, ok := danglingLink(filepath.Join(link, "a"), ws.dir)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, link, found)
	})

	t.Run("plain missing path has none", func(t *testing.T) {
		// Act
		_, ok := danglingLink(filepath.Join(ws.real, "missing", "a"), ws.dir)

		// Assert
		assert.False(t, ok)
	})
}

func Test_hasMountMeta(t *testing.T) {
	t.Run("expandable or splitting characters are found", func(t *testing.T) {
		for _, path := range []string{"/ws/x~", "/ws/$PWD", "/ws/a:/etc"} {
			// Act
			found := hasMountMeta(path)

			// Assert
			assert.True(t, found, path)
		}
	})

	t.Run("plain path has none", func(t *testing.T) {
		// Act
		found := hasMountMeta("/ws/real-dir.d")

		// Assert
		assert.False(t, found)
	})
}
