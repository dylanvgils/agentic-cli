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
			Host:          "agentic-proxy",
			Port:          "3128",
			NonProxyHosts: "localhost|127.0.0.1",
		},
		{
			ID:            "agentic-proxy-https",
			Active:        true,
			Protocol:      "https",
			Host:          "agentic-proxy",
			Port:          "3128",
			NonProxyHosts: "localhost|127.0.0.1",
		},
	}, settings.Proxies)
}
