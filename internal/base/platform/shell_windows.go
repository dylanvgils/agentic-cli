//go:build windows

package platform

// UsesPowerShell reports whether the host's default shell is PowerShell; true on Windows.
func UsesPowerShell() bool {
	return true
}
