package instructions

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// appendFile appends content to path, simulating a tool writing its own notes at runtime.
func appendFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = f.WriteString(content)
	return err
}

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
