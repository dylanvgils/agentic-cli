//go:build windows

package platform

// HostUser returns a 1000:1000 placeholder on Windows, which has no uid or gid.
func HostUser() User {
	return User{UID: 1000, GID: 1000}
}
