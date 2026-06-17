package buildinfo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsDev(t *testing.T) {
	t.Run("empty string is dev", func(t *testing.T) {
		// Act
		result := IsDev("")

		// Assert
		assert.True(t, result)
	})

	t.Run("dev is dev", func(t *testing.T) {
		// Act
		result := IsDev("dev")

		// Assert
		assert.True(t, result)
	})

	t.Run("released version is not dev", func(t *testing.T) {
		// Act
		result := IsDev("v1.2.3")

		// Assert
		assert.False(t, result)
	})
}

func TestIsDevBuild(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	t.Run("dev build", func(t *testing.T) {
		// Arrange
		Version = "dev"

		// Act
		result := IsDevBuild()

		// Assert
		assert.True(t, result)
	})

	t.Run("released build", func(t *testing.T) {
		// Arrange
		Version = "v1.2.3"

		// Act
		result := IsDevBuild()

		// Assert
		assert.False(t, result)
	})
}

func TestDevSourceDir(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	t.Run("released build returns empty without checking disk", func(t *testing.T) {
		// Arrange
		Version = "v1.2.3"

		// Act
		result := DevSourceDir("example.com/mod")

		// Assert
		assert.Empty(t, result)
	})

	t.Run("dev build delegates to findModuleRoot", func(t *testing.T) {
		// Arrange
		Version = "dev"
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/mod\n"), 0o644))
		t.Chdir(root)

		// Act
		result := DevSourceDir("example.com/mod")

		// Assert
		assert.Equal(t, root, result)
	})
}

func Test_findModuleRoot(t *testing.T) {
	t.Run("matches go.mod in current dir", func(t *testing.T) {
		// Arrange
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/mod\n"), 0o644))
		t.Chdir(root)

		// Act
		result := findModuleRoot("example.com/mod")

		// Assert
		assert.Equal(t, root, result)
	})

	t.Run("walks up past non-matching go.mod to matching ancestor", func(t *testing.T) {
		// Arrange
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/mod\n"), 0o644))
		nested := filepath.Join(root, "nested")
		require.NoError(t, os.Mkdir(nested, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(nested, "go.mod"), []byte("module example.com/other\n"), 0o644))
		t.Chdir(nested)

		// Act
		result := findModuleRoot("example.com/mod")

		// Assert
		assert.Equal(t, root, result)
	})

	t.Run("no matching go.mod returns empty", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())

		// Act
		result := findModuleRoot("example.com/nonexistent-module-xyz")

		// Assert
		assert.Empty(t, result)
	})
}

func Test_moduleMatches(t *testing.T) {
	t.Run("module declared on its own line", func(t *testing.T) {
		// Act
		result := moduleMatches([]byte("module example.com/mod\n\ngo 1.23\n"), "example.com/mod")

		// Assert
		assert.True(t, result)
	})

	t.Run("ignores surrounding whitespace", func(t *testing.T) {
		// Act
		result := moduleMatches([]byte("  module   example.com/mod  \n"), "example.com/mod")

		// Assert
		assert.True(t, result)
	})

	t.Run("different module returns false", func(t *testing.T) {
		// Act
		result := moduleMatches([]byte("module example.com/other\n"), "example.com/mod")

		// Assert
		assert.False(t, result)
	})

	t.Run("no module line returns false", func(t *testing.T) {
		// Act
		result := moduleMatches([]byte("go 1.23\n"), "example.com/mod")

		// Assert
		assert.False(t, result)
	})
}
