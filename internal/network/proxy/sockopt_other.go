//go:build !linux

package proxy

import (
	"syscall"
	"time"
)

// setUserTimeout is a no-op outside Linux.
func setUserTimeout(syscall.RawConn, time.Duration) {
	// Empty: the proxy only runs in a Linux container, but the CLI imports this package on every OS
}
