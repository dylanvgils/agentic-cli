//go:build integration

package container

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// skipWithoutCgroups skips when the daemon silently ignores resource limits (e.g. rootless without systemd).
func skipWithoutCgroups(t *testing.T) {
	t.Helper()
	out, err := exec.Command("docker", "info", "--format", "{{.CgroupDriver}}").Output()
	require.NoError(t, err)
	if strings.TrimSpace(string(out)) == "none" {
		t.Skip("docker daemon runs without cgroups, so resource limits aren't enforced")
	}
}

// writableScript prints <label>=rw when path can be created (and removes it again), <label>=ro otherwise.
func writableScript(label, path string) string {
	return "touch " + path + " 2>/dev/null && rm " + path + " && echo " + label + "=rw || echo " + label + "=ro"
}
