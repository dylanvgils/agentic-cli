package proxy

import (
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// setUserTimeout sets TCP_USER_TIMEOUT on raw; best effort, as the tunnel still works without it.
func setUserTimeout(raw syscall.RawConn, timeout time.Duration) {
	_ = raw.Control(func(fd uintptr) {
		_ = unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_USER_TIMEOUT, int(timeout.Milliseconds()))
	})
}
