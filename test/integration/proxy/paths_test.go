//go:build integration

package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestProxyPaths checks an allowed path on a path rule host reaches the upstream through decryption.
func TestProxyPaths(t *testing.T) {
	t.Parallel()

	// Arrange
	env := newServiceEnv(t)
	newProxyRun(t, env).allowingPaths(pathHost, "/org/repo/**").start()

	// Act
	out := env.curl("https://"+pathHost+"/org/repo/README.md", "--cacert", certsDir+"/proxy-ca.pem")

	// Assert
	assert.Contains(t, out, "path=/org/repo/README.md")
	assert.Contains(t, out, "status=200")
}
