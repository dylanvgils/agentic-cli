//go:build windows

package sandbox

import "golang.org/x/sys/windows"

// errAddrInUse is the error a listen on a port already in use fails with; syscall.EADDRINUSE is a made-up errno here.
const errAddrInUse = windows.WSAEADDRINUSE
