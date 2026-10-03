//go:build integration

package integration

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Mirrors internal/docker labels; duplicated so the tests stay black-box
const (
	labelProject    = "project=agentic-cli"
	labelOwner      = "agentic.owner"
	labelStarted    = "agentic.started"
	labelTimeFormat = "2006-01-02T15:04:05Z"
)

// fakeResource builds a labelled docker container or network that looks like agentic created it.
type fakeResource struct {
	t      *testing.T
	kind   string
	name   string
	labels []string
}

// fakeContainer starts building a stopped container from the test tool image.
func fakeContainer(t *testing.T) *fakeResource {
	return &fakeResource{t: t, kind: "container", labels: []string{labelProject}}
}

// fakeNetwork starts building a network.
func fakeNetwork(t *testing.T) *fakeResource {
	return &fakeResource{t: t, kind: "network", labels: []string{labelProject}}
}

// ownedBy labels the resource as a sidecar (network) of the owner container.
func (f *fakeResource) ownedBy(owner string) *fakeResource {
	f.labels = append(f.labels, labelOwner+"="+owner)
	return f
}

// startedAt labels when the resource was created.
func (f *fakeResource) startedAt(started time.Time) *fakeResource {
	f.labels = append(f.labels, labelStarted+"="+started.UTC().Format(labelTimeFormat))
	return f
}

// create creates the resource under a unique name and removes it on cleanup.
func (f *fakeResource) create() *fakeResource {
	f.t.Helper()
	f.name = uniqueName(f.t)

	args := []string{f.kind, "create"}
	for _, l := range f.labels {
		args = append(args, "--label", l)
	}

	if f.kind == "container" {
		args = append(args, "--name", f.name, testNamespace+"-"+testTool)
	} else {
		args = append(args, f.name)
	}

	docker(f.t, args...)
	f.t.Cleanup(func() { _ = exec.Command("docker", f.kind, "rm", "--force", f.name).Run() })
	return f
}

// exists reports whether the created resource is still there.
func (f *fakeResource) exists() bool {
	return exec.Command("docker", f.kind, "inspect", f.name).Run() == nil
}

// agenticCmd returns an agentic command running in workDir with the test AGENTIC_HOME and TMPDIR.
func agenticCmd(args ...string) *exec.Cmd {
	cmd := exec.Command(agenticBin, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "AGENTIC_HOME="+agenticHome, "TMPDIR="+tmpDir)
	return cmd
}

// runInTool runs script with sh inside the test tool container and returns the combined output.
func runInTool(t *testing.T, script string) string {
	t.Helper()
	out, err := agenticCmd("run", "--trust-dir", testTool, "--", "sh", "-c", script).CombinedOutput()
	require.NoError(t, err, string(out))
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
