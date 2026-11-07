//go:build integration

package egress

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
)

// TestEgressAllowlist checks the tool reaches allowed hosts only, through the run's proxy, which logs the rest.
func TestEgressAllowlist(t *testing.T) {
	startUpstream(t)

	forEachRuntime(t, func(t *testing.T, rt platform.Runtime) {
		// Arrange
		tool := platform.NewToolRun(t).
			On(rt).
			Allowing(allowedHost).
			WithProxy()

		// Act
		out := tool.Run(requestScript(allowedHost) + "; " + requestScript(otherHost))

		// Assert
		assert.Contains(t, out, "path=/egress")
		assert.Contains(t, out, "status=200")
		assert.Contains(t, out, "status=403")
		assert.Contains(t, proxyLog(t, tool), proxy.Entry{
			Protocol: proxy.ProtocolHTTP,
			Host:     otherHost,
			Port:     "80",
			Decision: proxy.DecisionDeny,
			Enforced: true,
		})
	})
}

// TestEgressMonitor checks monitor mode reaches the run's proxy, which then logs verdicts unenforced.
func TestEgressMonitor(t *testing.T) {
	startUpstream(t)

	forEachRuntime(t, func(t *testing.T, rt platform.Runtime) {
		// Arrange
		tool := platform.NewToolRun(t).
			On(rt).
			Allowing(allowedHost).
			WithProxyMonitor()

		// Act
		out := tool.Run(requestScript(allowedHost))

		// Assert
		assert.Contains(t, out, "status=200")
		assert.Contains(t, proxyLog(t, tool), proxy.Entry{
			Protocol: proxy.ProtocolHTTP,
			Host:     allowedHost,
			Port:     "80",
			Decision: proxy.DecisionAllow,
			Enforced: false,
		})
	})
}

// TestEgressBypass checks the tool can't reach an upstream directly, around the proxy.
func TestEgressBypass(t *testing.T) {
	upstream := startUpstream(t)
	script := "env -u HTTPS_PROXY -u https_proxy -u HTTP_PROXY -u http_proxy " +
		"curl -sS -m 5 -w '\\nstatus=%{http_code}\\n' http://" + upstream.IP() + "/egress; echo exit=$?"

	forEachRuntime(t, func(t *testing.T, rt platform.Runtime) {
		// Act
		out := platform.NewToolRun(t).
			On(rt).
			Allowing(allowedHost).
			WithProxy().
			Run(script)

		// Assert - only the upstream's own answer proves a bypass, not an error something on the way sends
		assert.NotContains(t, out, "path=/egress")
	})
}
