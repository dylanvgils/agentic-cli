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
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	workspace := t.TempDir()
	t.Chdir(workspace)
	target := filepath.Join(workspace, "real")
	require.NoError(t, os.MkdirAll(filepath.Join(target, "sub"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(target, "token"), []byte("test-secret"), 0o600))
	require.NoError(t, os.Symlink(target, filepath.Join(workspace, "link")))
	realTarget, err := filepath.EvalSymlinks(target)
	require.NoError(t, err)
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(workspace, "escape")))

	t.Run("workspace symlink is pinned to its target", func(t *testing.T) {
		// Arrange
		mounts := mountSet{volumes: []string{"$PWD/link:/data:ro"}, toolHome: t.TempDir(), containerHome: "/home/agent"}

		// Act
		pinned, err := mounts.pinSymlinks()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{realTarget + ":/data:ro"}, pinned.volumes)
	})

	t.Run("path below a workspace symlink is pinned", func(t *testing.T) {
		// Arrange
		mounts := mountSet{volumes: []string{"$PWD/link/sub:/data"}, toolHome: t.TempDir(), containerHome: "/home/agent"}

		// Act
		pinned, err := mounts.pinSymlinks()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{filepath.Join(realTarget, "sub") + ":/data"}, pinned.volumes)
	})

	t.Run("secret through a workspace symlink is pinned", func(t *testing.T) {
		// Arrange
		mounts := mountSet{secrets: []string{"token:$PWD/link/token"}, toolHome: t.TempDir(), containerHome: "/home/agent"}

		// Act
		pinned, err := mounts.pinSymlinks()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"token:" + filepath.Join(realTarget, "token")}, pinned.secrets)
	})

	t.Run("workspace symlink leading out of cwd is refused", func(t *testing.T) {
		// Arrange
		cases := []mountSet{
			{volumes: []string{"$PWD/escape:/workspace/.git:ro"}},
			{volumes: []string{"$PWD/escape/missing:/data"}},
			{secrets: []string{"key:$PWD/escape/id_ed25519"}},
		}

		for _, mounts := range cases {
			mounts.toolHome, mounts.containerHome = t.TempDir(), "/home/agent"

			// Act
			_, err := mounts.pinSymlinks()

			// Assert
			assert.ErrorContains(t, err, "leads through a workspace symlink")
		}
	})

	t.Run("plain paths, outside symlinks and named volumes stay as written", func(t *testing.T) {
		// Arrange
		outsideLink := filepath.Join(t.TempDir(), "link")
		require.NoError(t, os.Symlink(outside, outsideLink))
		volumes := []string{"$PWD:/workspace", outsideLink + ":/data", "agentic-cache:/cache", "$PWD/missing:/missing"}
		mounts := mountSet{volumes: volumes, toolHome: t.TempDir(), containerHome: "/home/agent"}

		// Act
		pinned, err := mounts.pinSymlinks()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, volumes, pinned.volumes)
	})
}

func Test_mountSet_checkConfigNotMounted(t *testing.T) {
	toolHome := t.TempDir()

	t.Run("tool subdir mounts are accepted", func(t *testing.T) {
		// Arrange
		volumes := []string{"$TOOL_HOME/tools/claude/data:$CONTAINER_HOME/.claude", t.TempDir() + ":/data"}
		mounts := mountSet{volumes: volumes, toolHome: toolHome, containerHome: "/home/agent"}

		// Act
		err := mounts.checkConfigNotMounted()

		// Assert
		assert.NoError(t, err)
	})

	t.Run("mounting the agentic home or a parent is refused", func(t *testing.T) {
		for _, volume := range []string{"$TOOL_HOME:/agentic", filepath.Dir(toolHome) + ":/parent:ro"} {
			// Arrange
			mounts := mountSet{volumes: []string{volume}, toolHome: toolHome, containerHome: "/home/agent"}

			// Act
			err := mounts.checkConfigNotMounted()

			// Assert
			assert.ErrorContains(t, err, "would expose "+config.ConfigFile(toolHome), volume)
		}
	})

	t.Run("secret pointing at agentic.json is refused", func(t *testing.T) {
		// Arrange
		mounts := mountSet{secrets: []string{"cfg:$TOOL_HOME/agentic.json"}, toolHome: toolHome, containerHome: "/home/agent"}

		// Act
		err := mounts.checkConfigNotMounted()

		// Assert
		assert.ErrorContains(t, err, "would expose")
	})

	t.Run("real path of a symlinked agentic home is refused before agentic.json exists", func(t *testing.T) {
		// Arrange
		if runtime.GOOS == "windows" {
			t.Skip("symlinks need privileges on windows")
		}
		realHome := t.TempDir()
		linkedHome := filepath.Join(t.TempDir(), "agentic")
		require.NoError(t, os.Symlink(realHome, linkedHome))
		mounts := mountSet{volumes: []string{realHome + ":/agentic"}, toolHome: linkedHome, containerHome: "/home/agent"}

		// Act
		err := mounts.checkConfigNotMounted()

		// Assert
		assert.ErrorContains(t, err, "would expose")
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
