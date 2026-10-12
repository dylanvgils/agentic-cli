//go:build !windows

package platform

// UsesPowerShell reports whether the host's default shell is PowerShell; false off Windows.
func UsesPowerShell() bool {
	return false
}
