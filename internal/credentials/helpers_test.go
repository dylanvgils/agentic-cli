package credentials

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeSecret writes content to a temp file and returns its path.
func writeSecret(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// stubHome points the user's home directory at a temp dir and returns it.
func stubHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

// stubPresets replaces the preset table with dummy presets for the test.
func stubPresets(t *testing.T, stub map[string]preset) {
	t.Helper()
	original := presets
	presets = stub
	t.Cleanup(func() { presets = original })
}
