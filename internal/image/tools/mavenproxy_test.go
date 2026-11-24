package tools

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_mavenSettingsFile(t *testing.T) {
	// Act
	file := mavenSettingsFile()

	// Assert
	assert.Equal(t, "/etc/agentic/maven/settings.xml", file.Dest)
	assert.Equal(t, "0644", file.Mode)
	var settings mavenSettings
	require.NoError(t, xml.Unmarshal([]byte(strings.Join(file.Lines, "\n")), &settings))
	assert.Equal(t, []mavenProxy{
		{
			ID:            "agentic-proxy-http",
			Active:        true,
			Protocol:      "http",
			Host:          "${env.AGENTIC_MAVEN_PROXY_HOST}",
			Port:          "@AGENTIC_MAVEN_PROXY_PORT@",
			Username:      "${env.AGENTIC_MAVEN_PROXY_USER}",
			Password:      "${env.AGENTIC_MAVEN_PROXY_PASSWORD}",
			NonProxyHosts: "localhost|127.0.0.1",
		},
		{
			ID:            "agentic-proxy-https",
			Active:        true,
			Protocol:      "https",
			Host:          "${env.AGENTIC_MAVEN_PROXY_HOST}",
			Port:          "@AGENTIC_MAVEN_PROXY_PORT@",
			Username:      "${env.AGENTIC_MAVEN_PROXY_USER}",
			Password:      "${env.AGENTIC_MAVEN_PROXY_PASSWORD}",
			NonProxyHosts: "localhost|127.0.0.1",
		},
	}, settings.Proxies)
}

func Test_mavenSettingsBlock(t *testing.T) {
	// Act
	block := mavenSettingsBlock()

	// Assert
	assert.Equal(t, []string{
		`if [[ -n "${AGENTIC_MAVEN_PROXY_PORT:-}" && -f /etc/agentic/maven/settings.xml ]]; then`,
		`  sed "s/@AGENTIC_MAVEN_PROXY_PORT@/${AGENTIC_MAVEN_PROXY_PORT}/" /etc/agentic/maven/settings.xml > /tmp/agentic-maven-settings.xml`,
		`  export MAVEN_ARGS="--global-settings /tmp/agentic-maven-settings.xml${MAVEN_ARGS:+ $MAVEN_ARGS}"`,
		"fi",
	}, block.Lines)
}
