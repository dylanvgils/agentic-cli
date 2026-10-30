//go:build integration

package proxy

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
)

// sniHost is the fake upstream's alias on agentic-net, listed exactly so the proxy may dial its private address.
const sniHost = "upstream.sni.test"

// TestProxyServerName checks the proxy refuses a tunnel to an allowed host whose TLS server name names another host.
func TestProxyServerName(t *testing.T) {
	// Arrange
	platform.FakeContainer(t).
		OnNetwork("agentic-net").
		WithAlias(sniHost).
		ListeningOn("443").
		Create()

	tool := platform.NewToolRun(t).
		Allowing(sniHost).
		WithProxy()

	// Act
	out := tool.Run(serverNameScript(sniHost, "other.example.test"))

	// Assert
	assert.NotContains(t, out, "exit=0")
	assert.Contains(t, proxyLog(t, tool), proxy.Entry{
		Protocol: proxy.ProtocolHTTPS,
		Host:     sniHost,
		Port:     "443",
		Decision: proxy.DecisionDeny,
		Enforced: true,
		Reason:   proxy.ReasonSNIMismatch,
	})
}
