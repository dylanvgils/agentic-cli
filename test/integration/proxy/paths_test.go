//go:build integration

package proxy

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
)

// TestProxyPaths checks allowed paths reach the upstream and others are refused and logged.
func TestProxyPaths(t *testing.T) {
	t.Parallel()

	// Arrange
	env := newServiceEnv(t)
	run := newProxyRun(t, env).allowingPaths(pathHost, "/org/repo/**").start()

	// Act
	allowed := env.curl("https://"+pathHost+"/org/repo/README.md", "--cacert", certsDir+"/proxy-ca.pem")
	refused := env.curl("https://"+pathHost+"/other/README.md", "--cacert", certsDir+"/proxy-ca.pem")

	// Assert
	assert.Contains(t, allowed, "path=/org/repo/README.md")
	assert.Contains(t, allowed, "status=200")
	assert.Contains(t, refused, "status=403")
	assert.NotContains(t, refused, "path=")
	assert.Contains(t, run.log(), platform.ProxyEntry{
		Protocol:    platform.ProtocolHTTPS,
		Host:        pathHost,
		Port:        "443",
		Decision:    platform.DecisionDeny,
		Enforced:    true,
		Intercepted: true,
		Path:        "/other/README.md",
		Reason:      platform.ReasonPathBlocked,
	})
}
