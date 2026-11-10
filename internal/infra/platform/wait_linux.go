//go:build linux

package platform

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

// ProcessHandle waits on a process through a pidfd, immune to pid reuse.
type ProcessHandle struct {
	fd int
	// pid is polled without pidfd support
	pid int
}

// OpenProcess takes a handle on pid while it runs.
func OpenProcess(pid int) (*ProcessHandle, error) {
	fd, err := unix.PidfdOpen(pid, 0)
	switch {
	case errors.Is(err, unix.ESRCH):
		return nil, ErrProcessGone
	// Kernels before 5.3, or a seccomp filter that blocks pidfd_open
	case errors.Is(err, unix.ENOSYS), errors.Is(err, unix.EPERM):
		return &ProcessHandle{fd: -1, pid: pid}, nil
	case err != nil:
		return nil, fmt.Errorf("open process %d: %w", pid, err)
	}
	return &ProcessHandle{fd: fd}, nil
}

// Wait blocks until the process exits.
func (h *ProcessHandle) Wait() error {
	if h.fd < 0 {
		pollUntilGone(h.pid)
		return nil
	}

	fds := []unix.PollFd{{Fd: int32(h.fd), Events: unix.POLLIN}}
	for {
		_, err := unix.Poll(fds, -1)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return fmt.Errorf("wait for process: %w", err)
		}
		return nil
	}
}

// Close releases the handle.
func (h *ProcessHandle) Close() error {
	if h.fd < 0 {
		return nil
	}
	return unix.Close(h.fd)
}
