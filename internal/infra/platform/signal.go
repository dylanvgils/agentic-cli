package platform

import (
	"os/signal"
	"syscall"
)

// IgnoreHangup keeps a detached process alive when its terminal closes or its stdout reader goes away.
func IgnoreHangup() {
	signal.Ignore(syscall.SIGHUP, syscall.SIGPIPE)
}
