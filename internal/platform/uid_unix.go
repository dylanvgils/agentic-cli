//go:build !windows

package platform

import (
	"os/user"
	"strconv"
)

// HostUser returns the current user's uid and gid, or 1000:1000 when they can't be read.
func HostUser() User {
	u, err := user.Current()
	if err != nil {
		return User{UID: 1000, GID: 1000}
	}

	uid, uidErr := strconv.Atoi(u.Uid)
	gid, gidErr := strconv.Atoi(u.Gid)
	if uidErr != nil || gidErr != nil {
		return User{UID: 1000, GID: 1000}
	}
	return User{UID: uid, GID: gid}
}
