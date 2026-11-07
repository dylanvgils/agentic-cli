//go:build integration

package proxy

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/stretchr/testify/assert"
)

// TestProxyForward checks plain HTTP reaches an allowed upstream and a denied host doesn't.
func TestProxyForward(t *testing.T) {
	env := newServiceEnv(t)
	run := newProxyRun(t, env).allowing(allowedHost).start()

	// The upstream has a private address, which an exact entry may reach
	t.Run("allowed host reaches the upstream and is logged", func(t *testing.T) {
		// Act
		out := env.curl("http://" + allowedHost + "/forward")

		// Assert
		assert.Contains(t, out, "path=/forward")
		assert.Contains(t, out, "status=200")
		assert.Contains(t, run.log(), proxy.Entry{
			Protocol: proxy.ProtocolHTTP,
			Host:     allowedHost,
			Port:     "80",
			Decision: proxy.DecisionAllow,
			Enforced: true,
		})
		assert.Contains(t, run.output(), "msg=request proto=http host="+allowedHost+" port=80 decision=allow")
	})

	t.Run("denied host is refused", func(t *testing.T) {
		// Act
		out := env.curl("http://" + deniedHost + "/")

		// Assert
		assert.Contains(t, out, "host not allowed by agentic proxy allowlist")
		assert.Contains(t, out, "status=403")
		assert.Contains(t, run.log(), proxy.Entry{
			Protocol: proxy.ProtocolHTTP,
			Host:     deniedHost,
			Port:     "80",
			Decision: proxy.DecisionDeny,
			Enforced: true,
		})
	})
}

// TestProxyTunnel checks HTTPS to an allowed host tunnels to the upstream's own TLS.
func TestProxyTunnel(t *testing.T) {
	// Arrange
	env := newServiceEnv(t)
	run := newProxyRun(t, env).allowing(allowedHost).start()

	// Act
	out := env.curl("https://"+allowedHost+"/tunnel", "--cacert", upstreamCA)

	// Assert
	assert.Contains(t, out, "path=/tunnel")
	assert.Contains(t, out, "status=200")
	assert.Contains(t, run.log(), proxy.Entry{
		Protocol: proxy.ProtocolHTTPS,
		Host:     allowedHost,
		Port:     "443",
		Decision: proxy.DecisionAllow,
		Enforced: true,
	})
}
