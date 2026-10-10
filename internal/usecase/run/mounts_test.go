package run

import (
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
	ws := chdirSymlinkWorkspace(t)
	base := buildRequest{toolConfig: tools.Configs["claude"], rc: &config.AgenticRC{}, containerHome: "/home/agent"}

	t.Run("checked mounts are kept as written", func(t *testing.T) {
		// Arrange
		req := base
		req.in = Input{ToolHome: t.TempDir(), Volumes: []string{"$PWD/real:/data"}}

		// Act
		mounts, err := newMountSet(req, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, mounts.volumes, "$PWD/real:/data:ro")
	})

	t.Run("mount through a workspace symlink is refused", func(t *testing.T) {
		// Arrange
		req := base
		req.in = Input{ToolHome: t.TempDir(), Volumes: []string{"$PWD/link:/data"}}

		// Act
		_, err := newMountSet(req, nil)

		// Assert
		assert.ErrorContains(t, err, "goes through workspace symlink "+filepath.Join(ws.dir, "link"))
	})
}

func Test_mountSet_checkMounts(t *testing.T) {
	chdirSymlinkWorkspace(t)
	toolHome := t.TempDir()

	t.Run("plain workspace mounts pass", func(t *testing.T) {
		// Arrange
		mounts := mountSet{volumes: []string{"$PWD:/workspace", "$PWD/real:/data"}, toolHome: toolHome, containerHome: "/home/agent"}

		// Act
		err := mounts.checkMounts()

		// Assert
		assert.NoError(t, err)
	})

	t.Run("mount exposing agentic.json is refused", func(t *testing.T) {
		// Arrange
		mounts := mountSet{volumes: []string{"$TOOL_HOME:/agentic"}, toolHome: toolHome, containerHome: "/home/agent"}

		// Act
		err := mounts.checkMounts()

		// Assert
		assert.ErrorContains(t, err, "would expose")
	})
}

func Test_mountSet_checkResolvable(t *testing.T) {
	toolHome := t.TempDir()

	t.Run("missing path is accepted", func(t *testing.T) {
		// Arrange
		mounts := mountSet{volumes: []string{filepath.Join(t.TempDir(), "missing", "dir") + ":/data"}, toolHome: toolHome, containerHome: "/home/agent"}

		// Act
		err := mounts.checkResolvable()

		// Assert
		assert.NoError(t, err)
	})

	t.Run("path that can't be resolved is refused", func(t *testing.T) {
		// Arrange
		host := stubUnresolvablePath(t)
		mounts := mountSet{volumes: []string{host + ":/h:rw"}, toolHome: toolHome, containerHome: "/home/agent"}

		// Act
		err := mounts.checkResolvable()

		// Assert
		assert.ErrorContains(t, err, "mount "+host+" can't be resolved")
	})

	t.Run("proc path is refused", func(t *testing.T) {
		// Arrange
		if runtime.GOOS == "windows" {
			t.Skip("no /proc on windows")
		}
		mounts := mountSet{secrets: []string{"key:/proc/1/root/home"}, toolHome: toolHome, containerHome: "/home/agent"}

		// Act
		err := mounts.checkResolvable()

		// Assert
		assert.EqualError(t, err, "mount /proc/1/root/home goes through /proc; mount the real path instead")
	})

	t.Run("link into proc is refused", func(t *testing.T) {
		// Arrange
		if runtime.GOOS != "linux" {
			t.Skip("needs /proc")
		}
		link := filepath.Join(t.TempDir(), "fd")
		require.NoError(t, os.Symlink("/proc/self/fd", link))
		mounts := mountSet{volumes: []string{link + ":/fd"}, toolHome: toolHome, containerHome: "/home/agent"}

		// Act
		err := mounts.checkResolvable()

		// Assert
		assert.ErrorContains(t, err, "goes through /proc")
	})
}

func Test_mountSet_checkSymlinks(t *testing.T) {
	ws := chdirSymlinkWorkspace(t)
	toolHome := t.TempDir()

	t.Run("secret through a workspace symlink is refused", func(t *testing.T) {
		// Arrange
		mounts := mountSet{secrets: []string{"key:$PWD/escape/id_ed25519"}, toolHome: toolHome, containerHome: "/home/agent"}

		// Act
		err := mounts.checkSymlinks()

		// Assert
		assert.EqualError(t, err, "mount "+filepath.Join(ws.dir, "escape", "id_ed25519")+" goes through workspace symlink "+filepath.Join(ws.dir, "escape")+"; mount the real path instead")
	})

	t.Run("named volume is not a path", func(t *testing.T) {
		// Arrange
		require.NoError(t, os.Symlink(ws.outside, filepath.Join(ws.dir, "agentic-cache")))
		mounts := mountSet{volumes: []string{"agentic-cache:/cache"}, toolHome: toolHome, containerHome: "/home/agent"}

		// Act
		err := mounts.checkSymlinks()

		// Assert
		assert.NoError(t, err)
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
	t.Run("missing path resolves through its existing parent", func(t *testing.T) {
		// Arrange
		ws := chdirSymlinkWorkspace(t)

		// Act
		real := resolveExisting(filepath.Join(ws.dir, "escape", "missing"))

		// Assert
		assert.Equal(t, filepath.Join(ws.outside, "missing"), real)
	})

	t.Run("unresolvable path is kept as is", func(t *testing.T) {
		// Arrange
		host := stubUnresolvablePath(t)

		// Act
		real := resolveExisting(host)

		// Assert
		assert.Equal(t, host, real)
	})
}

func Test_resolvePrefix(t *testing.T) {
	t.Run("missing path resolves through its existing parent", func(t *testing.T) {
		// Arrange
		ws := chdirSymlinkWorkspace(t)

		// Act
		real, err := resolvePrefix(filepath.Join(ws.dir, "escape", "missing", "leaf"))

		// Assert
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(ws.outside, "missing", "leaf"), real)
	})

	t.Run("error other than a missing path fails", func(t *testing.T) {
		// Arrange
		host := stubUnresolvablePath(t)

		// Act
		_, err := resolvePrefix(host)

		// Assert
		assert.ErrorIs(t, err, os.ErrPermission)
	})
}

func Test_isProcPath(t *testing.T) {
	t.Run("proc and paths below it match", func(t *testing.T) {
		// Act
		root, below := isProcPath("/proc"), isProcPath("/proc/1/root")

		// Assert
		assert.True(t, root)
		assert.True(t, below)
	})

	t.Run("name sharing the prefix does not match", func(t *testing.T) {
		// Act
		match := isProcPath("/processes")

		// Assert
		assert.False(t, match)
	})
}

func Test_workspaceSymlink(t *testing.T) {
	ws := chdirSymlinkWorkspace(t)

	t.Run("link that stays inside cwd is found", func(t *testing.T) {
		// Act
		link, ok := workspaceSymlink(filepath.Join(ws.dir, "link", "sub"), ws.dir)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, filepath.Join(ws.dir, "link"), link)
	})

	t.Run("dangling link is found", func(t *testing.T) {
		// Arrange
		dangling := filepath.Join(ws.dir, "dangling")
		require.NoError(t, os.Symlink(filepath.Join(ws.outside, "missing"), dangling))

		// Act
		link, ok := workspaceSymlink(filepath.Join(dangling, "deep"), ws.dir)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, dangling, link)
	})

	t.Run("plain path has none", func(t *testing.T) {
		// Act
		_, ok := workspaceSymlink(filepath.Join(ws.real, "missing"), ws.dir)

		// Assert
		assert.False(t, ok)
	})

	t.Run("link outside cwd is not checked", func(t *testing.T) {
		// Arrange
		outsideLink := filepath.Join(t.TempDir(), "link")
		require.NoError(t, os.Symlink(ws.outside, outsideLink))

		// Act
		_, ok := workspaceSymlink(outsideLink, ws.dir)

		// Assert
		assert.False(t, ok)
	})
}

func Test_workspaceRoot(t *testing.T) {
	cwd := t.TempDir()

	t.Run("path inside cwd has cwd as root", func(t *testing.T) {
		// Act
		root, ok := workspaceRoot(filepath.Join(cwd, "a"), cwd)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, cwd, root)
	})

	t.Run("path through an alias of cwd has the alias as root", func(t *testing.T) {
		// Arrange
		if runtime.GOOS == "windows" {
			t.Skip("symlinks need privileges on windows")
		}
		alias := filepath.Join(t.TempDir(), "alias")
		require.NoError(t, os.Symlink(cwd, alias))

		// Act
		root, ok := workspaceRoot(filepath.Join(alias, "a"), cwd)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, alias, root)
	})

	t.Run("case-insensitive filesystems ignore case", func(t *testing.T) {
		// Arrange
		stubCaseInsensitivePaths(t, true)

		// Act
		root, ok := workspaceRoot(filepath.Join(strings.ToUpper(cwd), "a"), cwd)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, strings.ToUpper(cwd), root)
	})

	t.Run("link back to cwd below it keeps the shallowest root", func(t *testing.T) {
		// Arrange
		if runtime.GOOS == "windows" {
			t.Skip("symlinks need privileges on windows")
		}
		require.NoError(t, os.Symlink(".", filepath.Join(cwd, "self")))

		// Act
		root, ok := workspaceRoot(filepath.Join(cwd, "self", "a"), cwd)

		// Assert
		assert.True(t, ok)
		assert.Equal(t, cwd, root)
	})

	t.Run("path outside cwd has no root", func(t *testing.T) {
		// Act
		_, ok := workspaceRoot(t.TempDir(), cwd)

		// Assert
		assert.False(t, ok)
	})
}
