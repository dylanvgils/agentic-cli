//go:build integration

package egress

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
)

// pathHost refuses HTTPS (see startRefusingHost), so an allowed path gets a 502 once the proxy has decrypted the request.
const pathHost = "paths.example.test"

// TestEgressPaths checks the proxy decrypts a path rule host, which the tool trusts, and refuses paths no rule allows.
func TestEgressPaths(t *testing.T) {
	startRefusingHost(t, pathHost)

	forEachRuntime(t, func(t *testing.T, rt platform.Runtime) {
		// Arrange
		tool := platform.NewToolRun(t).
			On(rt).
			AllowingPaths(pathHost, "/org/repo/**").
			WithProxy().
			ViaEntrypoint()

		// Act
		out := tool.Run(pathsScript())

		// Assert
		assert.Contains(t, out, "allowed=502")
		assert.Contains(t, out, "refused=403")
		assert.Contains(t, out, "proxy blocked 1 request(s) to paths no rule allows: "+pathHost+"/other/README.md")
		assert.Contains(t, proxyLog(t, tool), platform.ProxyEntry{
			Protocol:    platform.ProtocolHTTPS,
			Host:        pathHost,
			Port:        "443",
			Decision:    platform.DecisionDeny,
			Enforced:    true,
			Intercepted: true,
			Path:        "/other/README.md",
			Reason:      platform.ReasonPathBlocked,
		})
	})
}

// TestEgressCredentialPaths checks path rules limit a credential host, whose allowed paths still get the secret.
func TestEgressCredentialPaths(t *testing.T) {
	startRefusingHost(t, pathHost)

	forEachRuntime(t, func(t *testing.T, rt platform.Runtime) {
		// Arrange
		tool := platform.NewToolRun(t).
			On(rt).
			WithCredential(pathHost, secretFile(t)).
			AllowingPaths(pathHost, "/org/repo/**").
			ViaEntrypoint()

		// Act
		out := tool.Run(pathsScript())

		// Assert
		assert.Contains(t, out, "allowed=502")
		assert.Contains(t, out, "refused=403")
		assert.NotContains(t, out, "without the placeholder")
		assert.Contains(t, proxyLog(t, tool), platform.ProxyEntry{
			Protocol:    platform.ProtocolHTTPS,
			Host:        pathHost,
			Port:        "443",
			Decision:    platform.DecisionDeny,
			Enforced:    true,
			Intercepted: true,
			Path:        "/other/README.md",
			Reason:      platform.ReasonPathBlocked,
		})
	})
}
