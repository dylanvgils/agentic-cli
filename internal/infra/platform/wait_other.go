//go:build !linux && !darwin && !windows

package platform

// ProcessHandle polls a process.
type ProcessHandle struct {
	pid int
}

// OpenProcess takes a handle on pid while it runs.
func OpenProcess(pid int) (*ProcessHandle, error) {
	if !ProcessRunning(pid) {
		return nil, ErrProcessGone
	}
	return &ProcessHandle{pid: pid}, nil
}

// Wait blocks until the process exits.
func (h *ProcessHandle) Wait() error {
	pollUntilGone(h.pid)
	return nil
}

// Close releases the handle.
func (h *ProcessHandle) Close() error {
	return nil
}
