package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntegrationEnv(t *testing.T) {
	noAuth := ProxyEndpoint{
		Host: "proxy.example.test",
		Port: "3128",
	}
	withAuth := ProxyEndpoint{
		Host:     "proxy.example.test",
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
			name:        "maven passes the endpoint",
			integration: "maven",
			endpoint:    withAuth,
			want: []string{
				"AGENTIC_MAVEN_PROXY_HOST=proxy.example.test",
				"AGENTIC_MAVEN_PROXY_PORT=40123",
				"AGENTIC_MAVEN_PROXY_USER=agentic",
				"AGENTIC_MAVEN_PROXY_PASSWORD=test-secret",
			},
		},
		{
			name:        "gradle sets the proxy system properties",
			integration: "gradle",
			endpoint:    noAuth,
			want:        []string{"GRADLE_OPTS=-Dhttp.proxyHost=proxy.example.test -Dhttp.proxyPort=3128 -Dhttps.proxyHost=proxy.example.test -Dhttps.proxyPort=3128"},
		},
		{
			name:        "gradle passes the auth and allows Basic for tunnels",
			integration: "gradle",
			endpoint:    withAuth,
			want: []string{"GRADLE_OPTS=-Dhttp.proxyHost=proxy.example.test -Dhttp.proxyPort=40123 -Dhttp.proxyUser=agentic -Dhttp.proxyPassword=test-secret " +
				"-Dhttps.proxyHost=proxy.example.test -Dhttps.proxyPort=40123 -Dhttps.proxyUser=agentic -Dhttps.proxyPassword=test-secret " +
				"-Djdk.http.auth.tunneling.disabledSchemes="},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			env := IntegrationEnv([]string{tc.integration}, tc.endpoint)

			// Assert
			assert.Equal(t, tc.want, env)
		})
	}
}

func TestIntegrationEnvNames(t *testing.T) {
	// Act
	names := IntegrationEnvNames([]string{"gradle", "maven"})

	// Assert
	assert.Equal(t, []string{
		"GRADLE_OPTS",
		"AGENTIC_MAVEN_PROXY_HOST",
		"AGENTIC_MAVEN_PROXY_PORT",
		"AGENTIC_MAVEN_PROXY_USER",
		"AGENTIC_MAVEN_PROXY_PASSWORD",
	}, names)
}
