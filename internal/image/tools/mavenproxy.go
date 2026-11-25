package tools

import (
	"encoding/xml"
	"fmt"
	"strings"

	df "github.com/dylanvgils/agentic-cli/internal/image/dockerfile"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// mavenPortPlaceholder is the port in the settings template, as Maven can't read it from env.
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

// mavenSettingsFile writes the Maven proxy settings template; the token comes from env, never a file.
func mavenSettingsFile() df.Heredoc {
	var settings mavenSettings
	for _, protocol := range []string{"http", "https"} {
		settings.Proxies = append(settings.Proxies, mavenProxy{
			ID:            "agentic-proxy-" + protocol,
			Active:        true,
			Protocol:      protocol,
			Host:          mavenEnvRef(sandbox.MavenProxyHostEnv),
			Port:          mavenPortPlaceholder,
			Username:      mavenEnvRef(sandbox.MavenProxyUserEnv),
			Password:      mavenEnvRef(sandbox.MavenProxyPasswordEnv),
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

// integrationBlocks are the entrypoint blocks for the proxy integrations, adding Maven's settings only on the java extra.
func integrationBlocks(img ImageInfo) []df.Block {
	if img.HasExtra("java") {
		return []df.Block{mavenSettingsBlock()}
	}
	return nil
}

// mavenSettingsBlock renders the Maven settings with the proxy port and points Maven at them.
func mavenSettingsBlock() df.Block {
	return df.Block{
		Comment: "Render the Maven proxy settings for the maven integration",
		Lines: []string{
			fmt.Sprintf(`if [[ -n "${%s:-}" && -f %s ]]; then`, sandbox.MavenProxyPortEnv, mavenSettingsTemplatePath),
			fmt.Sprintf(`  sed "s/%s/${%s}/" %s > %s`,
				mavenPortPlaceholder,
				sandbox.MavenProxyPortEnv,
				mavenSettingsTemplatePath,
				mavenSettingsPath,
			),
			fmt.Sprintf(`  export MAVEN_ARGS="--global-settings %s${MAVEN_ARGS:+ $MAVEN_ARGS}"`, mavenSettingsPath),
			"fi",
		},
	}
}

// mavenEnvRef returns Maven's reference to env var name.
func mavenEnvRef(name string) string {
	return "${env." + name + "}"
}
