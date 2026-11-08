package buildinfo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsDev(t *testing.T) {
	for _, tc := range []struct {
		name    string
		version string
		want    bool
	}{
		{name: "empty string is dev", want: true},
		{name: "dev is dev", version: "dev", want: true},
		{name: "released version is not dev", version: "v1.2.3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := IsDev(tc.version)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func TestIsDevBuild(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	for _, tc := range []struct {
		name    string
		version string
		want    bool
	}{
		{name: "dev build", version: "dev", want: true},
		{name: "released build", version: "v1.2.3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			Version = tc.version

			// Act
			result := IsDevBuild()

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
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
		root := writeGoMod(t, t.TempDir(), "example.com/mod")
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
		root := writeGoMod(t, t.TempDir(), "example.com/mod")
		t.Chdir(root)

		// Act
		result := findModuleRoot("example.com/mod")

		// Assert
		assert.Equal(t, root, result)
	})

	t.Run("walks up past non-matching go.mod to matching ancestor", func(t *testing.T) {
		// Arrange
		root := writeGoMod(t, t.TempDir(), "example.com/mod")
		nested := writeGoMod(t, filepath.Join(root, "nested"), "example.com/other")
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
	for _, tc := range []struct {
		name  string
		gomod string
		want  bool
	}{
		{name: "module declared on its own line", gomod: "module example.com/mod\n\ngo 1.23\n", want: true},
		{name: "ignores surrounding whitespace", gomod: "  module   example.com/mod  \n", want: true},
		{name: "different module returns false", gomod: "module example.com/other\n"},
		{name: "no module line returns false", gomod: "go 1.23\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := moduleMatches([]byte(tc.gomod), "example.com/mod")

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

// writeGoMod writes a go.mod declaring module into dir, creating dir, and returns dir.
func writeGoMod(t *testing.T, dir, module string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module "+module+"\n"), 0o644))
	return dir
}
