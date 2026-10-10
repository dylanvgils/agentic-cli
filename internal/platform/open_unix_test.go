//go:build !windows

package platform

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenNoFollow(t *testing.T) {
	dir := t.TempDir()

	t.Run("symlink is refused", func(t *testing.T) {
		// Arrange
		target := filepath.Join(dir, "target")
		require.NoError(t, os.WriteFile(target, []byte("test-secret"), 0o600))
		link := filepath.Join(dir, "link")
		require.NoError(t, os.Symlink(target, link))

		// Act
		_, err := OpenNoFollow(link)

		// Assert
		assert.Error(t, err)
	})

	t.Run("fifo opens without blocking", func(t *testing.T) {
		// Arrange
		fifo := filepath.Join(dir, "fifo")
		require.NoError(t, syscall.Mkfifo(fifo, 0o600))

		// Act
		f, err := OpenNoFollow(fifo)

		// Assert
		require.NoError(t, err)
		_ = f.Close()
	})
}
