//go:build integration

package proxy

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
)

// guardHost is the fake upstream's alias on agentic-net, so it resolves to a private address.
const guardHost = "upstream.guard.test"

// TestProxyAddressGuard checks an allowed host can't reach local or private addresses unless listed exactly.
func TestProxyAddressGuard(t *testing.T) {
	platform.FakeContainer(t).
		OnNetwork("agentic-net").
		WithAlias(guardHost).
		ListeningOn("80").
		Create()

	t.Run("wildcard entry cannot reach a private address", func(t *testing.T) {
		// Arrange
		tool := platform.NewToolRun(t).
			Allowing(".guard.test").
			WithProxy()

		// Act
		out := tool.Run(connectScript(guardHost))

		// Assert
		assert.Contains(t, out, "connect=403")
		assert.Contains(t, proxyLog(t), `"host":"`+guardHost+`","port":"80","decision":"deny","enforced":true,"reason":"blocked-address"`)
	})

	t.Run("exact entry can reach a private address", func(t *testing.T) {
		// Arrange
		tool := platform.NewToolRun(t).
			Allowing(guardHost).
			WithProxy()

		// Act
		out := tool.Run(connectScript(guardHost))

		// Assert
		assert.Contains(t, out, "connect=200")
	})

	t.Run("exact entry cannot reach loopback", func(t *testing.T) {
		// Arrange
		tool := platform.NewToolRun(t).
			Allowing("localhost").
			WithProxy()

		// Act
		out := tool.Run(connectScript("localhost"))

		// Assert - the reason proves the guard refused it, not the allowlist
		assert.Contains(t, out, "connect=403")
		assert.Contains(t, proxyLog(t), `"host":"localhost","port":"80","decision":"deny","enforced":true,"reason":"blocked-address"`)
	})
}

// TestProxyClients checks a proxy without credentials also refuses clients outside its run's network.
func TestProxyClients(t *testing.T) {
	// Arrange
	tool := platform.NewToolRun(t).
		Allowing("example.test").
		WithProxy()
	var out string

	// Act
	tool.During(func() {
		out = curlFromEgressNet(t, tool.Proxy(), "http://example.test/")
	})

	// Assert
	assert.Contains(t, out, "client not on the agentic proxy network")
	assert.Contains(t, out, "status=403")
}
