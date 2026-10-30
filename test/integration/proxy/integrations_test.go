//go:build integration

package proxy

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
)

// Mirror internal/runtime/sandbox's proxy alias and the env the maven and gradle integrations set
const (
	mavenArgs  = "--global-settings /etc/agentic/maven/settings.xml"
	gradleOpts = "-Dhttp.proxyHost=agentic-proxy -Dhttp.proxyPort=3128 -Dhttps.proxyHost=agentic-proxy -Dhttps.proxyPort=3128"
)

// integrationsScript prints the integration env and whether the proxy address it names is reachable.
const integrationsScript = `echo "maven=$MAVEN_ARGS"; echo "gradle=$GRADLE_OPTS"; ` +
	`nc -z -w 3 agentic-proxy 3128 && echo proxy=reachable || echo proxy=unreachable`

// TestProxyIntegrations checks the maven and gradle integrations point the tools at the run's proxy, and only with --proxy.
func TestProxyIntegrations(t *testing.T) {
	t.Run("flag sets the env pointing at the proxy", func(t *testing.T) {
		// Act
		out := platform.NewToolRun(t).
			WithProxy().
			WithIntegrationFlag("maven", "gradle").
			Run(integrationsScript)

		// Assert
		assert.Contains(t, out, "maven="+mavenArgs+"\n")
		assert.Contains(t, out, "gradle="+gradleOpts+"\n")
		assert.Contains(t, out, "proxy=reachable")
	})

	t.Run("config sets the env pointing at the proxy", func(t *testing.T) {
		// Act
		out := platform.NewToolRun(t).
			Integrating("maven", "gradle").
			WithProxy().
			Run(integrationsScript)

		// Assert
		assert.Contains(t, out, "maven="+mavenArgs+"\n")
		assert.Contains(t, out, "gradle="+gradleOpts+"\n")
		assert.Contains(t, out, "proxy=reachable")
	})

	t.Run("without the proxy the env is unset", func(t *testing.T) {
		// Act
		out := platform.NewToolRun(t).
			WithIntegrationFlag("maven", "gradle").
			Run(integrationsScript)

		// Assert
		assert.Contains(t, out, "maven=\n")
		assert.Contains(t, out, "gradle=\n")
	})
}
