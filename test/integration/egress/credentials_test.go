//go:build integration

package egress

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
)

// credentialHost has no upstream, so the proxy answers 502 once it has decrypted the request.
const credentialHost = "upstream.example.test"

// TestEgressCredentials checks the tool sees only the placeholder and trusts the proxy CA for credential hosts.
func TestEgressCredentials(t *testing.T) {
	secretPath := secretFile(t)

	forEachRuntime(t, func(t *testing.T, rt platform.Runtime) {
		credentialRun := func(t *testing.T) *platform.ToolRun {
			return platform.NewToolRun(t).
				On(rt).
				WithCredential(credentialHost, secretPath)
		}

		t.Run("env holds the placeholder, not the secret", func(t *testing.T) {
			// Arrange
			script := `echo "key=$EXAMPLE_API_KEY"; env | grep -q '` + credentialSecret + `' && echo leak=yes || echo leak=no`

			// Act
			out := credentialRun(t).Run(script)

			// Assert
			assert.Contains(t, out, "agentic: injecting credentials for "+credentialHost+" ("+secretPath+")")
			assert.Regexp(t, `key=agentic-[0-9a-f]{24}\n`, out)
			assert.Contains(t, out, "leak=no")
		})

		t.Run("tool trusts the proxy CA for credential hosts", func(t *testing.T) {
			// Arrange
			script := `curl -sS -o /dev/null -w 'status=%{http_code}\n' https://` + credentialHost + `/ && echo tls=ok`
			tool := credentialRun(t).ViaEntrypoint()

			// Act
			out := tool.Run(script)

			// Assert
			assert.Contains(t, out, "status=502")
			assert.Contains(t, out, "tls=ok")
			assert.NotContains(t, out, "without the placeholder")
			assert.Contains(t, proxyLog(t, tool), proxy.Entry{
				Protocol:    proxy.ProtocolHTTPS,
				Host:        credentialHost,
				Port:        "443",
				Decision:    proxy.DecisionAllow,
				Enforced:    true,
				Intercepted: true,
			})
		})
	})
}
