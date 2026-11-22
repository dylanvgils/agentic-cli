package hostfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClass_modes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		class    Class
		wantDir  os.FileMode
		wantFile os.FileMode
	}{
		{
			name:     "data is user and group",
			class:    Data,
			wantDir:  0o750,
			wantFile: 0o640,
		},
		{
			name:     "private is user only",
			class:    Private,
			wantDir:  0o700,
			wantFile: 0o600,
		},
		{
			name:     "shared is world-readable",
			class:    Shared,
			wantDir:  0o755,
			wantFile: 0o644,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			dir := tc.class.Dir()
			file := tc.class.File()

			// Assert
			assert.Equal(t, tc.wantDir, dir)
			assert.Equal(t, tc.wantFile, file)
		})
	}
}

func TestClass_Dir(t *testing.T) {
	// Act + Assert
	assert.PanicsWithValue(t, "hostfs: unknown class 99", func() { Class(99).Dir() })
}

func TestClass_File(t *testing.T) {
	// Act + Assert
	assert.PanicsWithValue(t, "hostfs: unknown class 99", func() { Class(99).File() })
}

func TestClass_Mkdir(t *testing.T) {
	t.Run("creates dir with the class mode", func(t *testing.T) {
		// Arrange
		dir := filepath.Join(t.TempDir(), "run")

		// Act
		err := Private.Mkdir(dir)

		// Assert
		require.NoError(t, err)
		assertMode(t, dir, 0o700)
	})

	t.Run("fails when dir exists", func(t *testing.T) {
		// Act
		err := Private.Mkdir(t.TempDir())

		// Assert
		assert.ErrorIs(t, err, os.ErrExist)
	})
}

func TestClass_MkdirAll(t *testing.T) {
	// Arrange
	parent := filepath.Join(t.TempDir(), "parent")
	dir := filepath.Join(parent, "child")

	// Act
	err := Private.MkdirAll(dir)

	// Assert
	require.NoError(t, err)
	assertMode(t, parent, 0o700)
	assertMode(t, dir, 0o700)
}

func TestClass_WriteFile(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "secret")

	// Act
	err := Private.WriteFile(path, []byte("test-secret"))

	// Assert
	require.NoError(t, err)
	assertMode(t, path, 0o600)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "test-secret", string(content))
}

func TestClass_ReplaceFile(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o600))

	// Act
	err := Data.ReplaceFile(path, []byte("new"))

	// Assert
	require.NoError(t, err)
	assertMode(t, path, 0o640)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "new", string(content))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

// assertMode asserts path's permission bits.
func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, want, info.Mode().Perm())
}
