package tools

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

func TestProxyIntegrationEnv(t *testing.T) {
	sidecar := ProxyEndpoint{
		Host: "agentic-proxy",
		Port: "3128",
	}
	withAuth := ProxyEndpoint{
		Host:     "host.example.test",
		Port:     "40123",
		User:     "agentic",
		Password: "test-secret",
	}

	for _, tc := range []struct {
		name        string
		integration string
		endpoint    ProxyEndpoint
		want        []string
	}{
		{
			name:        "maven points at the rendered settings with the endpoint",
			integration: "maven",
			endpoint:    sidecar,
			want: []string{
				"MAVEN_ARGS=--global-settings /tmp/agentic-maven-settings.xml",
				"AGENTIC_MAVEN_PROXY_HOST=agentic-proxy",
				"AGENTIC_MAVEN_PROXY_PORT=3128",
				"AGENTIC_MAVEN_PROXY_USER=",
				"AGENTIC_MAVEN_PROXY_PASSWORD=",
			},
		},
		{
			name:        "maven passes the auth",
			integration: "maven",
			endpoint:    withAuth,
			want: []string{
				"MAVEN_ARGS=--global-settings /tmp/agentic-maven-settings.xml",
				"AGENTIC_MAVEN_PROXY_HOST=host.example.test",
				"AGENTIC_MAVEN_PROXY_PORT=40123",
				"AGENTIC_MAVEN_PROXY_USER=agentic",
				"AGENTIC_MAVEN_PROXY_PASSWORD=test-secret",
			},
		},
		{
			name:        "gradle sets the proxy system properties",
			integration: "gradle",
			endpoint:    sidecar,
			want:        []string{"GRADLE_OPTS=-Dhttp.proxyHost=agentic-proxy -Dhttp.proxyPort=3128 -Dhttps.proxyHost=agentic-proxy -Dhttps.proxyPort=3128"},
		},
		{
			name:        "gradle passes the auth and allows Basic for tunnels",
			integration: "gradle",
			endpoint:    withAuth,
			want: []string{"GRADLE_OPTS=-Dhttp.proxyHost=host.example.test -Dhttp.proxyPort=40123 -Dhttps.proxyHost=host.example.test -Dhttps.proxyPort=40123 " +
				"-Dhttp.proxyUser=agentic -Dhttp.proxyPassword=test-secret -Dhttps.proxyUser=agentic -Dhttps.proxyPassword=test-secret " +
				"-Djdk.http.auth.tunneling.disabledSchemes="},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			env := ProxyIntegrationEnv([]string{tc.integration}, tc.endpoint)

			// Assert
			assert.Equal(t, tc.want, env)
		})
	}

	t.Run("none for no integrations", func(t *testing.T) {
		// Act
		env := ProxyIntegrationEnv(nil, sidecar)

		// Assert
		assert.Empty(t, env)
	})
}

func TestProxyIntegrationEnvNames(t *testing.T) {
	// Act
	names := ProxyIntegrationEnvNames([]string{"gradle", "maven"})

	// Assert
	assert.Equal(t, []string{
		"GRADLE_OPTS",
		"MAVEN_ARGS",
		"AGENTIC_MAVEN_PROXY_HOST",
		"AGENTIC_MAVEN_PROXY_PORT",
		"AGENTIC_MAVEN_PROXY_USER",
		"AGENTIC_MAVEN_PROXY_PASSWORD",
	}, names)
}
