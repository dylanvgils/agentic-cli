//go:build !windows

package platform

import "syscall"

// detachedAttr starts a child in its own session.
func detachedAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
