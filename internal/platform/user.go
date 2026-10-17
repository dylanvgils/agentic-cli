package platform

import "fmt"

// User is a host uid and gid.
type User struct {
	UID int
	GID int
}

// String returns "uid:gid", as docker --user and chown take it.
func (u User) String() string {
	return fmt.Sprintf("%d:%d", u.UID, u.GID)
}
