//go:build !windows

package platform

import (
	"fmt"
	"os/user"
)

// GetUID returns the current user's UID.
func GetUID() string {
	u, err := user.Current()
	if err != nil {
		return "1000"
	}
	return u.Uid
}

// GetGID returns the current user's GID.
func GetGID() string {
	u, err := user.Current()
	if err != nil {
		return "1000"
	}
	return u.Gid
}

// UserGroup returns "UID:GID" for the --user docker flag.
func UserGroup() string {
	return fmt.Sprintf("%s:%s", GetUID(), GetGID())
}
