//go:build windows

package platform

import "syscall"

// detachedProcess starts a child without a console.
const detachedProcess = 0x00000008

// detachedAttr starts a child in its own process group, without a console.
func detachedAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess}
}
