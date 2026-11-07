//go:build integration

package proxy

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/stretchr/testify/assert"
)

// TestProxyServerName checks a tunnel to an allowed host whose TLS server name names another host is closed.
func TestProxyServerName(t *testing.T) {
	t.Parallel()

	// Arrange
	env := newServiceEnv(t)
	run := newProxyRun(t, env).allowing(allowedHost).start()

	// Act
	out := env.shell(serverNameScript(allowedHost, "other.example.test"))

	// Assert
	assert.NotContains(t, out, "exit=0")
	assert.Contains(t, run.log(), proxy.Entry{
		Protocol: proxy.ProtocolHTTPS,
		Host:     allowedHost,
		Port:     "443",
		Decision: proxy.DecisionDeny,
		Enforced: true,
		Reason:   proxy.ReasonSNIMismatch,
	})
}
