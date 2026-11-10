//go:build darwin

package platform

import (
	"errors"
	"fmt"
	"syscall"
)

// ProcessHandle waits on a process through kqueue, immune to pid reuse.
type ProcessHandle struct {
	kq int
}

// OpenProcess takes a handle on pid while it runs.
func OpenProcess(pid int) (*ProcessHandle, error) {
	kq, err := syscall.Kqueue()
	if err != nil {
		return nil, fmt.Errorf("create kqueue: %w", err)
	}

	change := syscall.Kevent_t{
		Ident:  uint64(pid),
		Filter: syscall.EVFILT_PROC,
		Flags:  syscall.EV_ADD | syscall.EV_ONESHOT,
		Fflags: syscall.NOTE_EXIT,
	}
	_, err = syscall.Kevent(kq, []syscall.Kevent_t{change}, nil, nil)
	if err != nil {
		_ = syscall.Close(kq)
		if errors.Is(err, syscall.ESRCH) {
			return nil, ErrProcessGone
		}
		return nil, fmt.Errorf("open process %d: %w", pid, err)
	}
	return &ProcessHandle{kq: kq}, nil
}

// Wait blocks until the process exits.
func (h *ProcessHandle) Wait() error {
	events := make([]syscall.Kevent_t, 1)
	for {
		n, err := syscall.Kevent(h.kq, nil, events, nil)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return fmt.Errorf("wait for process: %w", err)
		}
		if n > 0 {
			return nil
		}
	}
}

// Close releases the handle.
func (h *ProcessHandle) Close() error {
	return syscall.Close(h.kq)
}
