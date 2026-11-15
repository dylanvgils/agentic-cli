package proxy

import (
	"syscall"
	"time"
)

// setUserTimeout sets TCP_USER_TIMEOUT on raw; best effort, as the tunnel still works without it.
func setUserTimeout(raw syscall.RawConn, timeout time.Duration) {
	_ = raw.Control(func(fd uintptr) {
		_ = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_USER_TIMEOUT, int(timeout.Milliseconds()))
	})
}
