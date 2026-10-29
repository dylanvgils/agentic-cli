//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	// credentialHost has no upstream, so the proxy answers 502 once it has decrypted the request.
	credentialHost = "upstream.example.test"

	credentialSecret = "test-secret-value"

	// proxyCredentialsDir mirrors internal/runtime/docker's dir for the proxy's CA and credential files
	proxyCredentialsDir = "/run/agentic-proxy/credentials"
)

// TestProxyCredentials checks the tool sees only the placeholder and trusts the proxy CA for credential hosts.
func TestProxyCredentials(t *testing.T) {
	secretPath := secretFile(t)
	credentialRun := func(t *testing.T) *toolRun {
		return newToolRun(t).withCredential(credentialHost, secretPath)
	}

	t.Run("env holds the placeholder, not the secret", func(t *testing.T) {
		// Arrange
		script := `echo "key=$EXAMPLE_API_KEY"; env | grep -q '` + credentialSecret + `' && echo leak=yes || echo leak=no`

		// Act
		out := credentialRun(t).run(script)

		// Assert
		assert.Contains(t, out, "agentic: injecting credentials for "+credentialHost+" ("+secretPath+")")
		assert.Regexp(t, `key=agentic-[0-9a-f]{24}\n`, out)
		assert.Contains(t, out, "leak=no")
	})

	t.Run("tool trusts the proxy CA for credential hosts", func(t *testing.T) {
		// Arrange
		script := `curl -sS -o /dev/null -w 'status=%{http_code}\n' https://` + credentialHost + `/ && echo tls=ok`

		// Act
		out := credentialRun(t).
			viaEntrypoint().
			run(script)

		// Assert
		assert.Contains(t, out, "status=502")
		assert.Contains(t, out, "tls=ok")
		assert.NotContains(t, out, "without the placeholder")
		assert.Contains(t, proxyLog(t), `"host":"`+credentialHost+`","port":"443","decision":"allow","enforced":true,"intercepted":true`)
	})

	t.Run("proxy deletes the CA key and credentials once loaded", func(t *testing.T) {
		// Arrange
		var keyErr, credsErr, certErr error

		// Act
		credentialRun(t).during(func() {
			proxy := runningProxy(t)
			keyErr = copyFromProxy(proxy, "ca-key.pem")
			credsErr = copyFromProxy(proxy, "credentials.json")
			certErr = copyFromProxy(proxy, "ca.pem")
		})

		// Assert - the CA cert stays, which proves the path is right
		assert.Error(t, keyErr)
		assert.Error(t, credsErr)
		assert.NoError(t, certErr)
	})

	t.Run("proxy refuses clients outside the run's network", func(t *testing.T) {
		// Arrange
		var out string

		// Act
		credentialRun(t).during(func() {
			out = curlFromEgressNet(t, "http://"+credentialHost+"/")
		})

		// Assert
		assert.Contains(t, out, "client not on the agentic proxy network")
		assert.Contains(t, out, "status=403")
	})
}
