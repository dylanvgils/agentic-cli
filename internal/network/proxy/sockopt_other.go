//go:build !linux

package proxy

import (
	"syscall"
	"time"
)

// setUserTimeout does nothing: the proxy only runs in a Linux container, but the CLI imports this package on every OS.
func setUserTimeout(syscall.RawConn, time.Duration) {}
