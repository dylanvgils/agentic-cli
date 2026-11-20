package docker

import (
	"maps"
	"slices"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
)

func Test_proxyIntegrationEnv(t *testing.T) {
	// Act
	names := slices.Sorted(maps.Keys(proxyIntegrationEnv))

	// Assert
	assert.Equal(t, sandbox.EgressIntegrations(), names, "every egress integration has env, and only those")
}

func TestProxyIntegrationEnvNames(t *testing.T) {
	// Act
	names := ProxyIntegrationEnvNames([]string{"gradle", "maven"})

	// Assert
	assert.Equal(t, []string{"GRADLE_OPTS", "MAVEN_ARGS"}, names)
}

func Test_proxyIntegrationToolEnv(t *testing.T) {
	for _, tc := range []struct {
		name        string
		integration string
		want        []string
	}{
		{
			name:        "maven points at the global settings",
			integration: "maven",
			want:        []string{"MAVEN_ARGS=--global-settings /etc/agentic/maven/settings.xml"},
		},
		{
			name:        "gradle sets the proxy system properties",
			integration: "gradle",
			want:        []string{"GRADLE_OPTS=-Dhttp.proxyHost=agentic-proxy -Dhttp.proxyPort=3128 -Dhttps.proxyHost=agentic-proxy -Dhttps.proxyPort=3128"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			env := proxyIntegrationToolEnv([]string{tc.integration})

			// Assert
			assert.Equal(t, tc.want, env)
		})
	}

	t.Run("none for no integrations", func(t *testing.T) {
		// Act
		env := proxyIntegrationToolEnv(nil)

		// Assert
		assert.Empty(t, env)
	})
}
