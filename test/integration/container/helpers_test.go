//go:build integration

package container

import (
	"net"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/test/integration/platform"
)

// freePort returns a host port that's free on the loopback now; ports need a local daemon, so that's the daemon host.
func freePort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find a free port: %v", err)
	}
	defer func() { _ = listener.Close() }()
	return strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
}

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

// fetchOnDaemonHost returns the body at port on the daemon host's loopback, or "" when nothing answers; the daemon may be a
// sidecar, so it fetches from a container on the host network.
func fetchOnDaemonHost(port string) string {
	out, err := exec.Command("docker", "run", "--rm", "--network", "host",
		"--env", "http_proxy=",
		"--env", "HTTP_PROXY=",
		tools.BusyboxImageFor(""),
		"wget", "-qO-", "-T", "2", "http://127.0.0.1:"+port,
	).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
