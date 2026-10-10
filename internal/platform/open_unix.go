//go:build !windows

package platform

import (
	"os"
	"syscall"
)

// OpenNoFollow opens path read-only, failing on a symlink and returning at once on a FIFO instead of blocking.
func OpenNoFollow(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}
