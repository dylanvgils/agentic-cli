package proxy

import (
	"syscall"
	"time"
)

// tcpUserTimeout is TCP_USER_TIMEOUT, the same on every Linux arch; syscall only defines it on some.
const tcpUserTimeout = 0x12

// setUserTimeout sets TCP_USER_TIMEOUT on raw; best effort, as the tunnel still works without it.
func setUserTimeout(raw syscall.RawConn, timeout time.Duration) {
	_ = raw.Control(func(fd uintptr) {
		_ = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, tcpUserTimeout, int(timeout.Milliseconds()))
	})
}
