//go:build !windows

package sandbox

import "syscall"

// errAddrInUse is the error a listen on a port already in use fails with.
const errAddrInUse = syscall.EADDRINUSE
