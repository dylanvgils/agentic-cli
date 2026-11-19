//go:build integration

package container

import (
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
)

// freePort returns a port that's free on this machine's loopback now.
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

// skipWithRemoteDaemon skips when the docker context isn't on this machine (e.g. a DinD sidecar), where --port is refused.
func skipWithRemoteDaemon(t *testing.T) {
	t.Helper()
	host := platform.Docker(t, "context", "inspect", "--format", "{{.Endpoints.docker.Host}}")
	if !strings.HasPrefix(host, "unix://") && !strings.HasPrefix(host, "npipe://") {
		t.Skipf("docker daemon at %s isn't on this machine, so ports can't be forwarded", host)
	}
}

// writableScript prints <label>=rw when path can be created (and removes it again), <label>=ro otherwise.
func writableScript(label, path string) string {
	return "touch " + path + " 2>/dev/null && rm " + path + " && echo " + label + "=rw || echo " + label + "=ro"
}

// fetchLocal returns the body at port on this machine's loopback, or "" when nothing answers; it bypasses any proxy.
func fetchLocal(port string) string {
	client := &http.Client{
		Transport: &http.Transport{Proxy: nil},
		Timeout:   2 * time.Second,
	}
	resp, err := client.Get("http://127.0.0.1:" + port)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(body))
}
