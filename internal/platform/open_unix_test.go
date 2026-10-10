//go:build !windows

package platform

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenInRoot(t *testing.T) {
	// A FIFO the agent plants must not hang the read
	// Arrange
	dir := t.TempDir()
	require.NoError(t, syscall.Mkfifo(filepath.Join(dir, "fifo"), 0o600))
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })

	// Act
	f, err := OpenInRoot(root, "fifo")

	// Assert
	require.NoError(t, err)
	_ = f.Close()
}
