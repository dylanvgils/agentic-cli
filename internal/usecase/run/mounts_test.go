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

func Test_pinMountSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	workspace := t.TempDir()
	t.Chdir(workspace)
	target := t.TempDir()
	require.NoError(t, os.Symlink(target, filepath.Join(workspace, "link")))
	realTarget, err := filepath.EvalSymlinks(target)
	require.NoError(t, err)

	t.Run("workspace symlink is pinned to its target", func(t *testing.T) {
		// Act
		volumes, _ := pinMountSymlinks([]string{"$PWD/link:/data:ro"}, nil, t.TempDir(), "/home/agent")

		// Assert
		assert.Equal(t, []string{realTarget + ":/data:ro"}, volumes)
	})

	t.Run("path below a workspace symlink is pinned", func(t *testing.T) {
		// Arrange
		require.NoError(t, os.MkdirAll(filepath.Join(target, "sub"), 0o700))

		// Act
		volumes, _ := pinMountSymlinks([]string{"$PWD/link/sub:/data"}, nil, t.TempDir(), "/home/agent")

		// Assert
		assert.Equal(t, []string{filepath.Join(realTarget, "sub") + ":/data"}, volumes)
	})

	t.Run("secret through a workspace symlink is pinned", func(t *testing.T) {
		// Arrange
		require.NoError(t, os.WriteFile(filepath.Join(target, "token"), []byte("test-secret"), 0o600))

		// Act
		_, secrets := pinMountSymlinks(nil, []string{"token:$PWD/link/token"}, t.TempDir(), "/home/agent")

		// Assert
		assert.Equal(t, []string{"token:" + filepath.Join(realTarget, "token")}, secrets)
	})

	t.Run("plain paths, outside symlinks and named volumes stay as written", func(t *testing.T) {
		// Arrange
		outsideLink := filepath.Join(t.TempDir(), "link")
		require.NoError(t, os.Symlink(target, outsideLink))
		volumes := []string{"$PWD:/workspace", outsideLink + ":/data", "agentic-cache:/cache", "$PWD/missing:/missing"}

		// Act
		pinned, _ := pinMountSymlinks(volumes, nil, t.TempDir(), "/home/agent")

		// Assert
		assert.Equal(t, volumes, pinned)
	})
}

func Test_checkConfigNotMounted(t *testing.T) {
	toolHome := t.TempDir()

	t.Run("tool subdir mounts are accepted", func(t *testing.T) {
		// Arrange
		volumes := []string{"$TOOL_HOME/tools/claude/data:$CONTAINER_HOME/.claude", t.TempDir() + ":/data"}

		// Act
		err := checkConfigNotMounted(volumes, nil, toolHome, "/home/agent")

		// Assert
		assert.NoError(t, err)
	})

	t.Run("mounting the agentic home or a parent is refused", func(t *testing.T) {
		for _, volume := range []string{"$TOOL_HOME:/agentic", filepath.Dir(toolHome) + ":/parent:ro"} {
			// Act
			err := checkConfigNotMounted([]string{volume}, nil, toolHome, "/home/agent")

			// Assert
			assert.ErrorContains(t, err, "would expose "+config.ConfigFile(toolHome), volume)
		}
	})

	t.Run("secret pointing at agentic.json is refused", func(t *testing.T) {
		// Act
		err := checkConfigNotMounted(nil, []string{"cfg:$TOOL_HOME/agentic.json"}, toolHome, "/home/agent")

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

		// Act
		err := checkConfigNotMounted([]string{realHome + ":/agentic"}, nil, linkedHome, "/home/agent")

		// Assert
		assert.ErrorContains(t, err, "would expose")
	})
}

func Test_mountedHostPaths(t *testing.T) {
	// Arrange
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Act
	paths := mountedHostPaths([]string{"agentic-cache:/cache"}, []string{"token:~/token", "other:$HOME/other:/run/other"}, t.TempDir(), "/home/agent")

	// Assert
	assert.Equal(t, []string{filepath.Join(home, "token"), filepath.Join(home, "other")}, paths)
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
