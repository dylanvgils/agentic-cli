//go:build integration

package proxy

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/stretchr/testify/assert"
)

// TestProxyCredentials checks the upstream gets the secret while the client only ever sees the placeholder.
func TestProxyCredentials(t *testing.T) {
	env := newServiceEnv(t)
	run := newProxyRun(t, env).
		allowing(credentialHost).
		withCredential().
		start()

	t.Run("upstream gets the secret, client the placeholder", func(t *testing.T) {
		// Act
		out := env.curl("https://"+credentialHost+"/",
			"--cacert", certsDir+"/proxy-ca.pem",
			"--include",
			"--header", "Authorization: Bearer "+testPlaceholder,
		)

		// Assert
		assert.Contains(t, out, "secret=match")
		assert.Contains(t, out, "X-Echo-Authorization: Bearer "+testPlaceholder)
		assert.NotContains(t, out, testSecret)
		assert.Contains(t, run.log(), proxy.Entry{
			Protocol:    proxy.ProtocolHTTPS,
			Host:        credentialHost,
			Port:        "443",
			Decision:    proxy.DecisionAllow,
			Enforced:    true,
			Intercepted: true,
		})
	})

	t.Run("CA key and credentials are deleted once loaded", func(t *testing.T) {
		// Act
		keyLeft := run.credentialFileExists(proxy.CAKeyFile)
		credsLeft := run.credentialFileExists("credentials.json")
		certLeft := run.credentialFileExists(proxy.CACertFile)

		// Assert - the CA cert stays, which proves the dir is right
		assert.False(t, keyLeft)
		assert.False(t, credsLeft)
		assert.True(t, certLeft)
	})
}
