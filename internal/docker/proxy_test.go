package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartProxy(t *testing.T) {
	t.Run("creates internal network, hardened sidecar, and egress link", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, "network inspect")
		rs := RunSpec{ProxyImage: "default-proxy", ProxyAllow: []string{"api.anthropic.com"}, ProxyLogDir: "/tmp/agentic/proxy"}

		// Act
		handle, err := startProxy(rs)

		// Assert
		require.NoError(t, err)
		calls := get()

		var createArgs, runArgs, connectArgs []string
		for _, c := range calls {
			switch {
			case c.args[0] == "network" && c.args[1] == "create":
				createArgs = c.args
			case c.args[0] == "run":
				runArgs = c.args
			case c.args[0] == "network" && c.args[1] == "connect":
				connectArgs = c.args
			}
		}

		assert.Contains(t, createArgs, "--internal")
		assert.Contains(t, createArgs, handle.network)

		assert.Contains(t, runArgs, "--detach")
		assert.Contains(t, runArgs, "--read-only")
		assert.Contains(t, runArgs, "--cap-drop=ALL")
		assert.Contains(t, runArgs, "--security-opt=no-new-privileges:true")
		assert.Contains(t, runArgs, "--env=AGENTIC_PROXY_ALLOW=api.anthropic.com")
		assert.True(t, hasArgWithPrefix(runArgs, "--env=AGENTIC_PROXY_TZ_OFFSET="))
		assert.Equal(t, "default-proxy", runArgs[len(runArgs)-1])

		assert.Equal(t, []string{"network", "connect", NetworkName, handle.container}, connectArgs)
	})

	t.Run("removes network when sidecar fails to start", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, "network inspect", "run")
		rs := RunSpec{ProxyImage: "default-proxy", ProxyLogDir: "/tmp/agentic/proxy"}

		// Act
		_, err := startProxy(rs)

		// Assert
		require.Error(t, err)
		var sawNetworkRm bool
		for _, c := range get() {
			if c.args[0] == "network" && c.args[1] == "rm" {
				sawNetworkRm = true
			}
		}
		assert.True(t, sawNetworkRm, "expected network rm cleanup after failed run")
	})
}

func TestProxyHandleStop(t *testing.T) {
	t.Run("removes container and network", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)
		handle := proxyHandle{container: "agentic-proxy-abc", network: "agentic-proxy-abc"}

		// Act
		handle.Stop()

		// Assert
		calls := get()
		require.Len(t, calls, 2)
		assert.Equal(t, []string{"rm", "-f", "agentic-proxy-abc"}, calls[0].args)
		assert.Equal(t, []string{"network", "rm", "agentic-proxy-abc"}, calls[1].args)
	})
}

func TestProxyHandleEnvArgs(t *testing.T) {
	// Arrange
	handle := proxyHandle{container: "agentic-proxy-abc"}

	// Act
	args := handle.envArgs()

	// Assert
	assert.Contains(t, args, "--env=HTTPS_PROXY=http://agentic-proxy-abc:3128")
	assert.Contains(t, args, "--env=HTTP_PROXY=http://agentic-proxy-abc:3128")
	assert.Contains(t, args, "--env=NO_PROXY=localhost,127.0.0.1")
}

func TestProxyHandleDeniedHosts(t *testing.T) {
	t.Run("collects unique denied hosts and total", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		logPath := filepath.Join(dir, "run.jsonl")
		lines := strings.Join([]string{
			`{"time":"2026-06-17T12:00:00Z","host":"api.anthropic.com","port":"443","decision":"allow"}`,
			`{"time":"2026-06-17T12:00:01Z","host":"evil.com","port":"443","decision":"deny"}`,
			`{"time":"2026-06-17T12:00:02Z","host":"evil.com","port":"443","decision":"deny"}`,
			`{"time":"2026-06-17T12:00:03Z","host":"tracker.net","port":"443","decision":"deny"}`,
		}, "\n")
		require.NoError(t, os.WriteFile(logPath, []byte(lines+"\n"), 0o644))
		handle := proxyHandle{logPath: logPath}

		// Act
		hosts, total := handle.deniedHosts()

		// Assert
		assert.Equal(t, []string{"evil.com", "tracker.net"}, hosts)
		assert.Equal(t, 3, total)
	})

	t.Run("missing log yields nothing", func(t *testing.T) {
		// Arrange
		handle := proxyHandle{logPath: filepath.Join(t.TempDir(), "absent.jsonl")}

		// Act
		hosts, total := handle.deniedHosts()

		// Assert
		assert.Empty(t, hosts)
		assert.Zero(t, total)
	})
}
