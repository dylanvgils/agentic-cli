//go:build !windows

package platform

import (
	"errors"
	"syscall"
)

// ProcessRunning reports whether a process with pid exists; one owned by another user counts as running.
func ProcessRunning(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
