// Package dind generates the per-run files for the rootless Docker-in-Docker sidecar.
package dind

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
)

// Subordinate id range for inner containers, far above anything useradd hands out.
const (
	subIDStart = 2000000000
	subIDCount = 65536
)

// userName and Home match the upstream image's rootless user.
const (
	userName = "rootless"
	Home     = "/home/" + userName
)

// IdentityFiles are the generated /etc files mounted over the image's own.
var IdentityFiles = []string{"passwd", "group", "subuid", "subgid"}

// Identity is the host uid/gid the sidecar runs as.
type Identity struct {
	user platform.User
}

// NewIdentity returns the identity of host user u, refusing root since dockerd would then run rootful.
func NewIdentity(u platform.User) (Identity, error) {
	if u.UID == 0 {
		return Identity{}, fmt.Errorf("--dind refuses to run as root: the Docker sidecar would run as real root; run agentic as a regular user")
	}
	return Identity{user: u}, nil
}

// UserGroup returns "uid:gid" for --user.
func (id Identity) UserGroup() string {
	return id.user.String()
}

// files returns the generated /etc file contents, keyed by name.
func (id Identity) files() map[string]string {
	subIDs := fmt.Sprintf("%s:%d:%d\n", userName, subIDStart, subIDCount)

	uid, gid := strconv.Itoa(id.user.UID), strconv.Itoa(id.user.GID)

	group := "root:x:0:\n"
	if id.user.GID != 0 {
		group += userName + ":x:" + gid + ":\n"
	}

	return map[string]string{
		"passwd": "root:x:0:0:root:/root:/sbin/nologin\n" +
			userName + ":x:" + uid + ":" + gid + "::" + Home + ":/bin/sh\n",
		"group":  group,
		"subuid": subIDs,
		"subgid": subIDs,
	}
}

// WriteIdentityFiles writes id's generated /etc files into dir.
func WriteIdentityFiles(dir string, id Identity) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dind etc dir: %w", err)
	}

	for name, content := range id.files() {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return fmt.Errorf("write dind %s: %w", name, err)
		}
	}
	return nil
}
