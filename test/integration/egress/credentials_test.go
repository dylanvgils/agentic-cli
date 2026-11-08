//go:build integration

package egress

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
)

// credentialHost refuses HTTPS (see startRefusingHost), so the proxy answers 502 once it has decrypted the request.
const credentialHost = "upstream.example.test"

// TestEgressCredentials checks the tool sees only the placeholder and trusts the proxy CA for credential hosts.
func TestEgressCredentials(t *testing.T) {
	secretPath := secretFile(t)
	startRefusingHost(t, credentialHost)

	forEachRuntime(t, func(t *testing.T, rt platform.Runtime) {
		credentialRun := func(t *testing.T) *platform.ToolRun {
			return platform.NewToolRun(t).
				On(rt).
				WithCredential(credentialHost, secretPath)
		}

		t.Run("env holds the placeholder, not the secret", func(t *testing.T) {
			// Act
			out := credentialRun(t).Run(leakScript())

			// Assert
			assert.Contains(t, out, "agentic: injecting credentials for "+credentialHost+" ("+secretPath+")")
			assert.Regexp(t, `key=agentic-[0-9a-f]{24}\n`, out)
			assert.Contains(t, out, "leak=no")
		})

		t.Run("tool trusts the proxy CA for credential hosts", func(t *testing.T) {
			// Arrange
			tool := credentialRun(t).ViaEntrypoint()

			// Act
			out := tool.Run(trustScript(credentialHost))

			// Assert
			assert.Contains(t, out, "status=502")
			assert.Contains(t, out, "tls=ok")
			assert.NotContains(t, out, "without the placeholder")
			assert.Contains(t, proxyLog(t, tool), platform.ProxyEntry{
				Protocol:    platform.ProtocolHTTPS,
				Host:        credentialHost,
				Port:        "443",
				Decision:    platform.DecisionAllow,
				Enforced:    true,
				Intercepted: true,
			})
		})
	})
}

// TestEgressCredentialFiles checks the proxy agentic starts for a docker run deletes the CA key and credentials it got.
func TestEgressCredentialFiles(t *testing.T) {
	// Arrange
	tool := platform.NewToolRun(t).WithCredential(credentialHost, secretFile(t))
	var keyCopied, credsCopied, certCopied bool

	// Act
	tool.During(func() {
		proxy := tool.Proxy()
		keyCopied = copyFromProxy(proxy, "ca-key.pem")
		credsCopied = copyFromProxy(proxy, "credentials.json")
		certCopied = copyFromProxy(proxy, "ca.pem")
	})

	// Assert - the CA cert stays, which proves the path is right
	assert.False(t, keyCopied)
	assert.False(t, credsCopied)
	assert.True(t, certCopied)
}
