//go:build integration

package proxy

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/stretchr/testify/assert"
)

// TestProxyAddressGuard checks an allowed host can't reach private or loopback addresses unless listed exactly.
func TestProxyAddressGuard(t *testing.T) {
	t.Parallel()

	env := newServiceEnv(t)

	t.Run("wildcard entry cannot reach a private address", func(t *testing.T) {
		// Arrange
		run := newProxyRun(t, env).allowing(".service.test").start()

		// Act
		out := env.curl("http://" + allowedHost + "/")

		// Assert
		assert.Contains(t, out, "status=403")
		assert.Contains(t, run.log(), blockedEntry(allowedHost))
	})

	t.Run("exact entry cannot reach loopback", func(t *testing.T) {
		// Arrange
		run := newProxyRun(t, env).allowing("localhost").start()

		// Act
		out := env.curl("http://localhost/")

		// Assert
		assert.Contains(t, out, "status=403")
		assert.Contains(t, run.log(), blockedEntry("localhost"))
	})

	t.Run("monitor mode still refuses loopback", func(t *testing.T) {
		// Arrange
		run := newProxyRun(t, env).monitoring().start()

		// Act
		out := env.curl("http://localhost/")

		// Assert
		assert.Contains(t, out, "status=403")
		assert.Contains(t, run.log(), blockedEntry("localhost"))
	})
}

// blockedEntry is the log entry of a plain HTTP request to host the address guard refused.
func blockedEntry(host string) proxy.Entry {
	return proxy.Entry{
		Protocol: proxy.ProtocolHTTP,
		Host:     host,
		Port:     "80",
		Decision: proxy.DecisionDeny,
		Enforced: true,
		Reason:   proxy.ReasonBlockedAddr,
	}
}
