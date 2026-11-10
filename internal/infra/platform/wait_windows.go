//go:build windows

package platform

import (
	"errors"
	"fmt"
	"syscall"
)

// errInvalidParameter is OpenProcess's error for a gone pid.
const errInvalidParameter syscall.Errno = 87

// ProcessHandle waits on a process through its handle, immune to pid reuse.
type ProcessHandle struct {
	handle syscall.Handle
}

// OpenProcess takes a handle on pid while it runs.
func OpenProcess(pid int) (*ProcessHandle, error) {
	h, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, errInvalidParameter) {
		return nil, ErrProcessGone
	}
	if err != nil {
		return nil, fmt.Errorf("open process %d: %w", pid, err)
	}
	return &ProcessHandle{handle: h}, nil
}

// Wait blocks until the process exits.
func (h *ProcessHandle) Wait() error {
	if _, err := syscall.WaitForSingleObject(h.handle, syscall.INFINITE); err != nil {
		return fmt.Errorf("wait for process: %w", err)
	}
	return nil
}

// Close releases the handle.
func (h *ProcessHandle) Close() error {
	return syscall.CloseHandle(h.handle)
}
