//go:build integration

package proxy

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/stretchr/testify/assert"
)

// TestProxyToken checks a token-protected proxy serves only clients sending the token.
func TestProxyToken(t *testing.T) {
	env := newServiceEnv(t)
	newProxyRun(t, env).
		allowing(allowedHost).
		withToken().
		start()

	t.Run("client without the token is refused", func(t *testing.T) {
		// Act
		out := env.curl("http://" + allowedHost + "/")

		// Assert
		assert.Contains(t, out, "status=407")
	})

	t.Run("client with the token is served", func(t *testing.T) {
		// Act
		out := env.curl("http://"+allowedHost+"/", "--proxy-user", proxy.AuthUser+":"+testToken)

		// Assert
		assert.Contains(t, out, "status=200")
	})
}
