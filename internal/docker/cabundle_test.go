package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_systemCABundle(t *testing.T) {
	t.Run("cache hit returns the cached bundle without creating a container", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		cachePath := filepath.Join(toolHome, caBundleCacheDir, "abc123.crt")
		require.NoError(t, os.MkdirAll(filepath.Dir(cachePath), 0o700))
		require.NoError(t, os.WriteFile(cachePath, []byte(testSystemBundle), 0o644))
		get := stubCABundleDocker(t, "sha256:abc123\n", testSystemBundle)

		// Act
		bundle, err := systemCABundle(toolHome, "agentic-claude")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, testSystemBundle, string(bundle))
		assert.Nil(t, findCall(get(), "create"))
	})

	t.Run("cache miss extracts and caches the bundle, then removes the container", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		get := stubCABundleDocker(t, "sha256:abc123\n", testSystemBundle)

		// Act
		bundle, err := systemCABundle(toolHome, "agentic-claude")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, testSystemBundle, string(bundle))
		calls := get()
		assert.Contains(t, findCall(calls, "create"), "--label=project=agentic-cli", "clean must find a leftover container")
		assert.Contains(t, findCall(calls, "cp"), "stub-container:"+systemCABundlePath)
		assert.Equal(t, []string{"rm", "--force", "stub-container"}, findCall(calls, "rm"))
		assert.FileExists(t, filepath.Join(toolHome, caBundleCacheDir, "abc123.crt"))
		entries, err := os.ReadDir(filepath.Join(toolHome, caBundleCacheDir))
		require.NoError(t, err)
		assert.Len(t, entries, 1, "no temp file is left behind")
	})

	t.Run("a bundle without certificates is an error and not cached", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		get := stubCABundleDocker(t, "sha256:abc123\n", "not a bundle")

		// Act
		_, err := systemCABundle(toolHome, "agentic-claude")

		// Assert
		require.ErrorContains(t, err, "holds no certificates")
		assert.NoFileExists(t, filepath.Join(toolHome, caBundleCacheDir, "abc123.crt"))
		assert.NotNil(t, findCall(get(), "rm"))
	})

	t.Run("an unexpected image id is an error", func(t *testing.T) {
		// Arrange
		stubCABundleDocker(t, "../../escape\n", testSystemBundle)

		// Act
		_, err := systemCABundle(t.TempDir(), "agentic-claude")

		// Assert
		require.ErrorContains(t, err, "unexpected image id")
	})

	t.Run("inspect error propagates", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", fmt.Errorf("stub: no such image"))

		// Act
		_, err := systemCABundle(t.TempDir(), "agentic-claude")

		// Assert
		require.ErrorContains(t, err, "no such image")
	})
}

func TestRemoveCABundleCache(t *testing.T) {
	// Arrange
	toolHome := t.TempDir()
	cacheDir := filepath.Join(toolHome, caBundleCacheDir)
	require.NoError(t, os.MkdirAll(cacheDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "abc123.crt"), []byte(testSystemBundle), 0o644))

	// Act
	err := RemoveCABundleCache(toolHome)

	// Assert
	require.NoError(t, err)
	assert.NoDirExists(t, cacheDir)
}

func Test_isHexDigest(t *testing.T) {
	t.Run("lowercase hex is accepted", func(t *testing.T) {
		// Act
		result := isHexDigest("0123456789abcdef")

		// Assert
		assert.True(t, result)
	})

	t.Run("empty, uppercase and path characters are rejected", func(t *testing.T) {
		for _, id := range []string{"", "ABC", "ab/cd", "..", "sha256:ab"} {
			// Act
			result := isHexDigest(id)

			// Assert
			assert.False(t, result, id)
		}
	})
}
