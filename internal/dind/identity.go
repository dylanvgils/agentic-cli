// Package dind generates the per-run files the rootless Docker-in-Docker sidecar needs: TLS certs, a seccomp profile and /etc identity files.
package dind

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The sidecar's subordinate id range: inner containers' ids map here on the host. It sits far
// above anything useradd hands out (SUB_UID_MAX defaults to 600100000), so it belongs to nobody.
const (
	subIDStart = 2000000000
	subIDCount = 65536
)

// userName and Home match the upstream image's rootless user, whose home holds the data-root volume.
const (
	userName = "rootless"
	Home     = "/home/" + userName
)

// IdentityFiles are the /etc files generated per run and bind-mounted read-only over the image's own.
var IdentityFiles = []string{"passwd", "group", "subuid", "subgid"}

// Identity is the host uid/gid the sidecar runs as, matching the tool container's --user.
type Identity struct {
	uid string
	gid string
}

// NewIdentity parses the host "uid:gid" the sidecar runs as, refusing root: as uid 0 the image
// would start dockerd in its rootful mode, with real root behind the sidecar's capabilities.
func NewIdentity(userGroup string) (Identity, error) {
	uid, gid, _ := strings.Cut(userGroup, ":")
	if uid == "0" {
		return Identity{}, fmt.Errorf("--dind refuses to run as root: the Docker sidecar would run as real root; run agentic as a regular user")
	}
	return Identity{uid: uid, gid: gid}, nil
}

// UserGroup returns "uid:gid" for --user.
func (id Identity) UserGroup() string {
	return id.uid + ":" + id.gid
}

// files returns the contents of the generated /etc files: a passwd/group entry so newuidmap can
// resolve the host uid, and subuid/subgid granting it only the dedicated subIDStart range.
func (id Identity) files() map[string]string {
	subIDs := fmt.Sprintf("%s:%d:%d\n", userName, subIDStart, subIDCount)

	group := "root:x:0:\n"
	if id.gid != "0" {
		group += userName + ":x:" + id.gid + ":\n"
	}

	return map[string]string{
		"passwd": "root:x:0:0:root:/root:/sbin/nologin\n" +
			userName + ":x:" + id.uid + ":" + id.gid + "::" + Home + ":/bin/sh\n",
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
