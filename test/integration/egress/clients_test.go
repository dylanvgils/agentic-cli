//go:build integration

package egress

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
)

// TestEgressClientNets checks a docker run's proxy refuses clients outside the run's network, with or without
// credentials.
func TestEgressClientNets(t *testing.T) {
	cases := map[string]func(t *testing.T) *platform.ToolRun{
		"without credentials": func(t *testing.T) *platform.ToolRun {
			return platform.NewToolRun(t).Allowing(credentialHost).WithProxy()
		},
		"with credentials": func(t *testing.T) *platform.ToolRun {
			return platform.NewToolRun(t).WithCredential(credentialHost, secretFile(t))
		},
	}

	for name, newRun := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			tool := newRun(t)
			var out string

			// Act
			tool.During(func() {
				out = curlFromEgressNet(t, tool.Proxy(), "http://"+credentialHost+"/")
			})

			// Assert
			assert.Contains(t, out, "client not on the agentic proxy network")
			assert.Contains(t, out, "status=403")
		})
	}
}

// TestEgressProxyToken checks a microsandbox run's proxy, published on host loopback, refuses requests without the
// run's token.
func TestEgressProxyToken(t *testing.T) {
	// Arrange
	rt := microsandbox(t)
	startUpstream(t)

	// Act
	out := platform.NewToolRun(t).
		On(rt).
		Allowing(allowedHost).
		WithProxy().
		Run(tokenlessScript(allowedHost))

	// Assert
	assert.Contains(t, out, "status=407")
	assert.Contains(t, out, "connect=407")
}
