package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/platform"
)

// The sidecar's subordinate id range: inner containers' ids map here on the host. It sits far
// above anything useradd hands out (SUB_UID_MAX defaults to 600100000), so it belongs to nobody.
const (
	dindSubIDStart = 2000000000
	dindSubIDCount = 65536
)

// dindUserName and dindHome match the upstream image's rootless user, whose home holds the data-root volume.
const (
	dindUserName = "rootless"
	dindHome     = "/home/" + dindUserName
)

// dindIdentityFiles are the /etc files generated per run and bind-mounted read-only over the image's own.
var dindIdentityFiles = []string{"passwd", "group", "subuid", "subgid"}

// hostUserGroup is a test-stubbable indirection into platform.UserGroup.
var hostUserGroup = platform.UserGroup

// dindIdentity is the host uid/gid the sidecar runs as, matching the tool container's --user.
type dindIdentity struct {
	uid string
	gid string
}

// currentDindIdentity returns the host user the sidecar runs as, refusing root: as uid 0 the image
// would start dockerd in its rootful mode, with real root behind the sidecar's capabilities.
func currentDindIdentity() (dindIdentity, error) {
	uid, gid, _ := strings.Cut(hostUserGroup(), ":")
	if uid == "0" {
		return dindIdentity{}, fmt.Errorf("--dind refuses to run as root: the Docker sidecar would run as real root; run agentic as a regular user")
	}
	return dindIdentity{uid: uid, gid: gid}, nil
}

// userGroup returns "uid:gid" for --user.
func (id dindIdentity) userGroup() string {
	return id.uid + ":" + id.gid
}

// files returns the contents of the generated /etc files: a passwd/group entry so newuidmap can
// resolve the host uid, and subuid/subgid granting it only the dedicated dindSubIDStart range.
func (id dindIdentity) files() map[string]string {
	subIDs := fmt.Sprintf("%s:%d:%d\n", dindUserName, dindSubIDStart, dindSubIDCount)

	group := "root:x:0:\n"
	if id.gid != "0" {
		group += dindUserName + ":x:" + id.gid + ":\n"
	}

	return map[string]string{
		"passwd": "root:x:0:0:root:/root:/sbin/nologin\n" +
			dindUserName + ":x:" + id.uid + ":" + id.gid + "::" + dindHome + ":/bin/sh\n",
		"group":  group,
		"subuid": subIDs,
		"subgid": subIDs,
	}
}

// writeDindIdentityFiles writes id's generated /etc files into dir.
func writeDindIdentityFiles(dir string, id dindIdentity) error {
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
