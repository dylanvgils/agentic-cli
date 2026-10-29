package tools

import (
	"encoding/xml"
	"strings"

	df "github.com/dylanvgils/agentic-cli/internal/image/dockerfile"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// mavenSettings is the part of Maven's settings.xml agentic sets.
type mavenSettings struct {
	XMLName xml.Name     `xml:"settings"`
	Proxies []mavenProxy `xml:"proxies>proxy"`
}

// mavenProxy is a settings.xml <proxy> entry.
type mavenProxy struct {
	ID            string `xml:"id"`
	Active        bool   `xml:"active"`
	Protocol      string `xml:"protocol"`
	Host          string `xml:"host"`
	Port          string `xml:"port"`
	NonProxyHosts string `xml:"nonProxyHosts"`
}

// mavenSettingsFile writes the Maven settings for the maven proxy integration, one proxy per scheme.
func mavenSettingsFile() df.Heredoc {
	var settings mavenSettings
	for _, protocol := range []string{"http", "https"} {
		settings.Proxies = append(settings.Proxies, mavenProxy{
			ID:            "agentic-proxy-" + protocol,
			Active:        true,
			Protocol:      protocol,
			Host:          sandbox.ProxyHostAlias,
			Port:          sandbox.ProxyPort,
			NonProxyHosts: "localhost|127.0.0.1",
		})
	}
	// Plain structs always marshal
	data, _ := xml.MarshalIndent(settings, "", "  ")

	return df.Heredoc{
		Dest:  sandbox.MavenSettingsPath,
		Lines: strings.Split(string(data), "\n"),
		Mode:  "0644",
	}
}
