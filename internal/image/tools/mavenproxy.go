package tools

import (
	"encoding/xml"
	"strings"

	df "github.com/dylanvgils/agentic-cli/internal/image/dockerfile"
)

// mavenPortPlaceholder stands in for the proxy port in the settings template, as Maven only reads a number there.
const mavenPortPlaceholder = "@AGENTIC_MAVEN_PROXY_PORT@"

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
	Username      string `xml:"username"`
	Password      string `xml:"password"`
	NonProxyHosts string `xml:"nonProxyHosts"`
}

// mavenSettingsFile writes the Maven settings template for the maven proxy integration, one proxy per scheme. Maven
// reads the endpoint from the env mavenProxyEnv sets, so the token never lands in a file; mavenSettingsBlock fills in
// the port.
func mavenSettingsFile() df.Heredoc {
	var settings mavenSettings
	for _, protocol := range []string{"http", "https"} {
		settings.Proxies = append(settings.Proxies, mavenProxy{
			ID:            "agentic-proxy-" + protocol,
			Active:        true,
			Protocol:      protocol,
			Host:          mavenEnvRef(MavenProxyHostEnv),
			Port:          mavenPortPlaceholder,
			Username:      mavenEnvRef(MavenProxyUserEnv),
			Password:      mavenEnvRef(MavenProxyPasswordEnv),
			NonProxyHosts: "localhost|127.0.0.1",
		})
	}
	// Plain structs always marshal
	data, _ := xml.MarshalIndent(settings, "", "  ")

	return df.Heredoc{
		Dest:  mavenSettingsTemplatePath,
		Lines: strings.Split(string(data), "\n"),
		Mode:  "0644",
	}
}

// mavenSettingsBlock is the entrypoint block that renders the Maven settings template with the proxy port, in java
// images with the maven integration on.
func mavenSettingsBlock() df.Block {
	return df.Block{Comment: "Fill in the egress proxy's port for the maven integration", Lines: []string{
		`if [[ -n "${` + MavenProxyPortEnv + `:-}" && -f ` + mavenSettingsTemplatePath + ` ]]; then`,
		`  sed "s/` + mavenPortPlaceholder + `/${` + MavenProxyPortEnv + `}/" ` + mavenSettingsTemplatePath + ` > ` + mavenSettingsPath,
		"fi",
	}}
}

// mavenEnvRef returns Maven's settings reference to env var name.
func mavenEnvRef(name string) string {
	return "${env." + name + "}"
}
