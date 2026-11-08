//go:build integration

package proxy

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
)

// TestProxyAddressGuard checks the guard refuses private and loopback addresses, over HTTP and CONNECT.
func TestProxyAddressGuard(t *testing.T) {
	t.Parallel()

	env := newServiceEnv(t)

	t.Run("wildcard entry cannot reach a private address", func(t *testing.T) {
		// Arrange
		run := newProxyRun(t, env).allowing(".service.test").start()

		// Act
		out := env.curl("http://" + allowedHost + "/")
		connect := env.connect(allowedHost)

		// Assert
		assert.Contains(t, out, "status=403")
		assert.Contains(t, connect, "connect=403")
		assert.Contains(t, run.log(), blockedEntry(platform.ProtocolHTTP, allowedHost))
		assert.Contains(t, run.log(), blockedEntry(platform.ProtocolHTTPS, allowedHost))
	})

	t.Run("exact entry cannot reach loopback", func(t *testing.T) {
		// Arrange
		run := newProxyRun(t, env).allowing("localhost").start()

		// Act
		out := env.curl("http://localhost/")
		connect := env.connect("localhost")

		// Assert
		assert.Contains(t, out, "status=403")
		assert.Contains(t, connect, "connect=403")
		assert.Contains(t, run.log(), blockedEntry(platform.ProtocolHTTP, "localhost"))
		assert.Contains(t, run.log(), blockedEntry(platform.ProtocolHTTPS, "localhost"))
	})

	t.Run("monitor mode still refuses loopback", func(t *testing.T) {
		// Arrange
		run := newProxyRun(t, env).monitoring().start()

		// Act
		out := env.curl("http://localhost/")
		connect := env.connect("localhost")

		// Assert
		assert.Contains(t, out, "status=403")
		assert.Contains(t, connect, "connect=403")
		assert.Contains(t, run.log(), blockedEntry(platform.ProtocolHTTP, "localhost"))
		assert.Contains(t, run.log(), blockedEntry(platform.ProtocolHTTPS, "localhost"))
	})
}

// blockedEntry is the log entry of a request to host the address guard refused.
func blockedEntry(protocol, host string) platform.ProxyEntry {
	port := "80"
	if protocol == platform.ProtocolHTTPS {
		port = "443"
	}
	return platform.ProxyEntry{
		Protocol: protocol,
		Host:     host,
		Port:     port,
		Decision: platform.DecisionDeny,
		Enforced: true,
		Reason:   platform.ReasonBlockedAddr,
	}
}
