package run

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
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

func Test_mountSet_pinSymlinks(t *testing.T) {
	ws := chdirSymlinkWorkspace(t)
	toolHome := t.TempDir()

	t.Run("volumes and secrets are pinned", func(t *testing.T) {
		// Arrange
		mounts := mountSet{volumes: []string{"$PWD/link:/data"}, secrets: []string{"token:$PWD/link/token"}, toolHome: toolHome, containerHome: "/home/agent"}

		// Act
		pinned, err := mounts.pinSymlinks()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{ws.real + ":/data"}, pinned.volumes)
		assert.Equal(t, []string{"token:" + filepath.Join(ws.real, "token")}, pinned.secrets)
	})

	t.Run("escaping secret is refused", func(t *testing.T) {
		// Arrange
		mounts := mountSet{secrets: []string{"key:$PWD/escape/id_ed25519"}, toolHome: toolHome, containerHome: "/home/agent"}

		// Act
		_, err := mounts.pinSymlinks()

		// Assert
		assert.ErrorContains(t, err, "leads through a workspace symlink")
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
