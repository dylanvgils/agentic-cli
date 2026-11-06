//go:build windows

package platform

import "os"

// ProcessRunning reports whether a process with pid exists, which FindProcess checks by opening it.
func ProcessRunning(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	_ = p.Release()
	return true
}
