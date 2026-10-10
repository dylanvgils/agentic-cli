//go:build !windows

package platform

import (
	"os"
	"syscall"
)

// OpenInRoot opens name in root read-only, returning at once on a FIFO instead of blocking; root follows links that stay inside it, so callers compare the opened file with what they checked.
func OpenInRoot(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
