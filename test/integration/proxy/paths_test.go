//go:build integration

package proxy

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
)

// pathHost has no upstream, so an allowed path gets a 502 once the proxy has decrypted the request.
const pathHost = "paths.example.test"

// TestProxyPaths checks the proxy decrypts a path rule host, which the tool trusts, and refuses paths no rule allows.
func TestProxyPaths(t *testing.T) {
	// Arrange
	tool := platform.NewToolRun(t).
		AllowingPaths(pathHost, "/org/repo/**").
		WithProxy().
		ViaEntrypoint()

	// Act
	out := tool.Run(pathsScript())

	// Assert
	assert.Contains(t, out, "allowed=502")
	assert.Contains(t, out, "refused=403")
	assert.Contains(t, out, "proxy blocked 1 request(s) to paths no rule allows: "+pathHost+"/other/README.md")
	assert.Contains(t, proxyLog(t), `"host":"`+pathHost+`","port":"443","decision":"deny","enforced":true,"intercepted":true,"path":"/other/README.md","reason":"path-blocked"`)
}

// TestProxyCredentialPaths checks path rules limit a credential host, whose allowed paths still get the secret.
func TestProxyCredentialPaths(t *testing.T) {
	// Arrange
	tool := platform.NewToolRun(t).
		WithCredential(pathHost, secretFile(t)).
		AllowingPaths(pathHost, "/org/repo/**").
		ViaEntrypoint()

	// Act
	out := tool.Run(pathsScript())

	// Assert
	assert.Contains(t, out, "allowed=502")
	assert.Contains(t, out, "refused=403")
	assert.NotContains(t, out, "without the placeholder")
	assert.Contains(t, proxyLog(t), `"host":"`+pathHost+`","port":"443","decision":"deny","enforced":true,"intercepted":true,"path":"/other/README.md","reason":"path-blocked"`)
}
