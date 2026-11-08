package instructions

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// symlinkOrSkip creates link pointing at target, skipping the test where symlinks need privileges.
func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	require.NoError(t, os.Symlink(target, link))
}

// newHostPath returns a fresh tools dir and the host path of name inside it.
func newHostPath(t *testing.T, name ...string) (string, string) {
	t.Helper()
	toolsDir := t.TempDir()
	return toolsDir, filepath.Join(append([]string{toolsDir}, name...)...)
}

// openTestRoot opens a fresh temp dir as a root, closed on cleanup.
func openTestRoot(t *testing.T) (*os.Root, string) {
	t.Helper()
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })
	return root, dir
}

// managed returns block wrapped in the managed markers, as written to a file.
func managed(block string) string {
	return instructionsBeginMarker + "\n" + block + "\n" + instructionsEndMarker + "\n"
}

// readFile returns path's content, failing the test on error.
func readFile(t *testing.T, path string) string {
	t.Helper()
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(got)
}

// writeTemp writes content to a new file in a temp dir and returns its path.
func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o640))
	return path
}
