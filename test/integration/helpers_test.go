//go:build integration

package integration

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// agenticCmd returns an agentic command running in workDir with the test AGENTIC_HOME and TMPDIR.
func agenticCmd(args ...string) *exec.Cmd {
	cmd := exec.Command(agenticBin, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "AGENTIC_HOME="+agenticHome, "TMPDIR="+tmpDir)
	return cmd
}

// runInTool runs script with sh inside the test tool container, passing flags to agentic run, and returns the combined output.
func runInTool(t *testing.T, script string, flags ...string) string {
	t.Helper()
	args := append([]string{"run", "--trust-dir"}, flags...)
	args = append(args, testTool, "--", "sh", "-c", script)

	out, err := agenticCmd(args...).CombinedOutput()
	t.Logf("agentic %s\n%s", strings.Join(args, " "), out)
	require.NoError(t, err)
	return string(out)
}

// skipWithoutCgroups skips when the daemon silently ignores resource limits (e.g. rootless without systemd).
func skipWithoutCgroups(t *testing.T) {
	t.Helper()
	out, err := exec.Command("docker", "info", "--format", "{{.CgroupDriver}}").Output()
	require.NoError(t, err)
	if strings.TrimSpace(string(out)) == "none" {
		t.Skip("docker daemon runs without cgroups, so resource limits aren't enforced")
	}
}

// tempDirIn creates a dir under parent, removed on cleanup; use it over t.TempDir for anything bind-mounted.
func tempDirIn(t *testing.T, parent string) string {
	t.Helper()
	dir, err := os.MkdirTemp(parent, "itest-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// uniqueName returns a docker resource name that won't clash with real agentic resources.
func uniqueName(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 6)
	_, err := rand.Read(buf)
	require.NoError(t, err)
	return "agentic-itest-" + hex.EncodeToString(buf)
}

// docker runs a docker command and fails the test on error.
func docker(t *testing.T, args ...string) {
	t.Helper()
	out, err := exec.Command("docker", args...).CombinedOutput()
	require.NoError(t, err, string(out))
}
