//go:build windows

package platform

import "syscall"

// createNoWindow gives a child a hidden console, which the docker and msb children it starts inherit instead of each
// opening a window.
const createNoWindow = 0x08000000

// detachedAttr starts a child in its own process group, with a hidden console of its own.
func detachedAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindow}
}
