//go:build integration

package egress

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
)

// integrationsScript prints the proxy the integration env names and whether it is reachable.
const integrationsScript = `echo "maven=$AGENTIC_MAVEN_PROXY_USER@$AGENTIC_MAVEN_PROXY_HOST"; echo "gradle=$GRADLE_OPTS"; ` +
	`nc -z -w 3 "$AGENTIC_MAVEN_PROXY_HOST" "$AGENTIC_MAVEN_PROXY_PORT" && echo proxy=reachable || echo proxy=unreachable`

// TestEgressIntegrations checks the integrations point the tool at the run's proxy, only with --proxy.
func TestEgressIntegrations(t *testing.T) {
	forEachRuntime(t, func(t *testing.T, rt platform.Runtime) {
		// Arrange
		host, user := "host.microsandbox.internal", "agentic"
		if rt.Docker() {
			host, user = "agentic-proxy", ""
		}

		// Act
		out := platform.NewToolRun(t).
			On(rt).
			WithProxy().
			WithIntegrationFlag("maven", "gradle").
			Run(integrationsScript)

		// Assert
		assert.Contains(t, out, "maven="+user+"@"+host+"\n")
		assert.Contains(t, out, "-Dhttps.proxyHost="+host)
		assert.Contains(t, out, "proxy=reachable")
	})

	t.Run("config sets the env", func(t *testing.T) {
		// Act
		out := platform.NewToolRun(t).
			Integrating("maven", "gradle").
			WithProxy().
			Run(integrationsScript)

		// Assert
		assert.Contains(t, out, "maven=@agentic-proxy\n")
		assert.Contains(t, out, "-Dhttps.proxyHost=agentic-proxy")
	})

	t.Run("without the proxy the env is unset", func(t *testing.T) {
		// Act
		out := platform.NewToolRun(t).
			WithIntegrationFlag("maven", "gradle").
			Run(integrationsScript)

		// Assert
		assert.Contains(t, out, "maven=@\n")
		assert.Contains(t, out, "gradle=\n")
	})
}
