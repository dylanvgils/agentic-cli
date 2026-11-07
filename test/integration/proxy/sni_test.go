//go:build integration

package proxy

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/stretchr/testify/assert"
)

// TestProxyServerName checks a tunnel to an allowed host whose TLS server name names another host is closed.
func TestProxyServerName(t *testing.T) {
	t.Parallel()

	// Arrange
	env := newServiceEnv(t)
	run := newProxyRun(t, env).allowing(allowedHost).start()
	script := "echo | timeout 5 openssl s_client -proxy " + proxyAlias + ":" + tools.ProxyPort +
		" -connect " + allowedHost + ":443 -servername other.example.test 2>&1; echo exit=$?"

	// Act
	out := env.shell(script)

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
