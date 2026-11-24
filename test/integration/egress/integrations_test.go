//go:build integration

package egress

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
)

// Mirror internal/image/tools' proxy alias and the env the maven and gradle integrations set
const (
	mavenArgs  = "--global-settings /tmp/agentic-maven-settings.xml"
	mavenProxy = "agentic-proxy:3128"
	gradleOpts = "-Dhttp.proxyHost=agentic-proxy -Dhttp.proxyPort=3128 -Dhttps.proxyHost=agentic-proxy -Dhttps.proxyPort=3128"
)

// integrationsScript prints the integration env and whether the proxy address it names is reachable.
const integrationsScript = `echo "maven=$MAVEN_ARGS"; echo "gradle=$GRADLE_OPTS"; ` +
	`echo "maven-proxy=$AGENTIC_MAVEN_PROXY_HOST:$AGENTIC_MAVEN_PROXY_PORT"; ` +
	`nc -z -w 3 "$AGENTIC_MAVEN_PROXY_HOST" "$AGENTIC_MAVEN_PROXY_PORT" && echo proxy=reachable || echo proxy=unreachable`

// TestEgressIntegrations checks the maven and gradle integrations point the tools at the run's Docker proxy, and only
// with --proxy.
func TestEgressIntegrations(t *testing.T) {
	t.Run("flag sets the env pointing at the proxy", func(t *testing.T) {
		// Act
		out := platform.NewToolRun(t).
			WithProxy().
			WithIntegrationFlag("maven", "gradle").
			Run(integrationsScript)

		// Assert
		assert.Contains(t, out, "maven="+mavenArgs+"\n")
		assert.Contains(t, out, "maven-proxy="+mavenProxy+"\n")
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
		assert.Contains(t, out, "maven-proxy="+mavenProxy+"\n")
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

// TestEgressIntegrationsMicrosandbox checks the integrations point the tools at a microsandbox run's published proxy,
// with its token.
func TestEgressIntegrationsMicrosandbox(t *testing.T) {
	// Arrange
	rt := microsandbox(t)

	// Act
	out := platform.NewToolRun(t).
		On(rt).
		WithProxy().
		WithIntegrationFlag("maven", "gradle").
		Run(integrationsScript + `; echo "maven-user=$AGENTIC_MAVEN_PROXY_USER"`)

	// Assert
	assert.Contains(t, out, "maven="+mavenArgs+"\n")
	assert.Contains(t, out, "maven-proxy=host.microsandbox.internal:")
	assert.Contains(t, out, "maven-user=agentic\n")
	assert.Contains(t, out, "-Dhttps.proxyHost=host.microsandbox.internal")
	assert.Contains(t, out, "proxy=reachable")
}
