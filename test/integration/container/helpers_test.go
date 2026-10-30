//go:build integration

package container

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
)

// skipWithoutCgroups skips when the daemon silently ignores resource limits (e.g. rootless without systemd).
func skipWithoutCgroups(t *testing.T) {
	t.Helper()
	if platform.Docker(t, "info", "--format", "{{.CgroupDriver}}") == "none" {
		t.Skip("docker daemon runs without cgroups, so resource limits aren't enforced")
	}
}

// writableScript prints <label>=rw when path can be created (and removes it again), <label>=ro otherwise.
func writableScript(label, path string) string {
	return "touch " + path + " 2>/dev/null && rm " + path + " && echo " + label + "=rw || echo " + label + "=ro"
}
