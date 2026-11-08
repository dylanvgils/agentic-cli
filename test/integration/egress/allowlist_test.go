//go:build integration

package egress

import (
	"testing"

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
		script := requestScript(allowedHost) + "; " + requestScript(otherHost) + "; " + tunnelScript(allowedHost)

		// Act
		out := tool.Run(script)

		// Assert
		assert.Contains(t, out, "path=/egress")
		assert.Contains(t, out, "status=200")
		assert.Contains(t, out, "status=403")
		assert.Contains(t, out, "path=/tunnel")
		assert.Contains(t, proxyLog(t, tool), platform.ProxyEntry{
			Protocol: platform.ProtocolHTTP,
			Host:     otherHost,
			Port:     "80",
			Decision: platform.DecisionDeny,
			Enforced: true,
		})
	})
}

// TestEgressAddressGuard checks the run's proxy refuses a wildcard-allowed host on a private address.
func TestEgressAddressGuard(t *testing.T) {
	startUpstream(t)

	forEachRuntime(t, func(t *testing.T, rt platform.Runtime) {
		// Arrange
		tool := platform.NewToolRun(t).
			On(rt).
			Allowing(".egress.test").
			WithProxy()

		// Act
		out := tool.Run(requestScript(allowedHost))

		// Assert
		assert.Contains(t, out, "status=403")
		assert.Contains(t, proxyLog(t, tool), platform.ProxyEntry{
			Protocol: platform.ProtocolHTTP,
			Host:     allowedHost,
			Port:     "80",
			Decision: platform.DecisionDeny,
			Enforced: true,
			Reason:   platform.ReasonBlockedAddr,
		})
	})
}

// TestEgressMonitor checks monitor mode logs verdicts without enforcing them.
func TestEgressMonitor(t *testing.T) {
	startUpstream(t)

	forEachRuntime(t, func(t *testing.T, rt platform.Runtime) {
		monitorRun := func(t *testing.T) *platform.ToolRun {
			return platform.NewToolRun(t).
				On(rt).
				Allowing(allowedHost).
				WithProxyMonitor()
		}

		t.Run("allowed host is logged unenforced", func(t *testing.T) {
			// Arrange
			tool := monitorRun(t)

			// Act
			out := tool.Run(requestScript(allowedHost))

			// Assert
			assert.Contains(t, out, "status=200")
			assert.Contains(t, proxyLog(t, tool), platform.ProxyEntry{
				Protocol: platform.ProtocolHTTP,
				Host:     allowedHost,
				Port:     "80",
				Decision: platform.DecisionAllow,
				Enforced: false,
			})
		})

		// A public host, as the address guard refuses non-allowed private ones even in monitor mode
		t.Run("other host gets through and is logged", func(t *testing.T) {
			// Arrange
			platform.SkipWithoutInternet(t, "example.org")
			tool := monitorRun(t)

			// Act
			out := tool.Run(reachableScript("other", "example.org"))

			// Assert
			assert.Contains(t, out, "other=ok")
			assert.Contains(t, proxyLog(t, tool), platform.ProxyEntry{
				Protocol: platform.ProtocolHTTPS,
				Host:     "example.org",
				Port:     "443",
				Decision: platform.DecisionDeny,
				Enforced: false,
			})
		})
	})
}

// TestEgressBypass checks the tool reaches nothing around the proxy: no upstream, no public host.
func TestEgressBypass(t *testing.T) {
	upstream := startUpstream(t)

	forEachRuntime(t, func(t *testing.T, rt platform.Runtime) {
		// Act
		out := platform.NewToolRun(t).
			On(rt).
			Allowing(allowedHost).
			WithProxy().
			Run(bypassScript(upstream.IP()))

		// Assert - only the upstream's own answer proves a bypass, not an error something on the way sends
		assert.Contains(t, out, "exit=")
		assert.NotContains(t, out, "path=/egress")
		assert.Contains(t, out, "public=blocked")
	})
}
