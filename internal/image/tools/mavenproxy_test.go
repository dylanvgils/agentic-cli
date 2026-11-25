package tools

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"

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
	dir := t.TempDir()
	vars := defaultScriptVars()
	vars.MavenSettingsTemplate = filepath.Join(dir, "template.xml")
	vars.MavenSettings = filepath.Join(dir, "settings.xml")
	require.NoError(t, os.WriteFile(vars.MavenSettingsTemplate, []byte("<port>"+mavenPortPlaceholder+"</port>\n"), 0o644))
	probe := printVars("MAVEN_ARGS")

	t.Run("renders the port and puts the settings before the user's args", func(t *testing.T) {
		// Act
		out, _ := runScript(t, "mavensettings.sh", vars, probe, sandbox.MavenProxyPortEnv+"=3128", "MAVEN_ARGS=-Dtest=1")

		// Assert
		assert.Equal(t, "MAVEN_ARGS=--global-settings "+vars.MavenSettings+" -Dtest=1\n", out)
		assert.Equal(t, "<port>3128</port>\n", readFile(t, vars.MavenSettings))
	})

	t.Run("without the port changes nothing", func(t *testing.T) {
		// Arrange
		resetFiles(t, vars.MavenSettings)

		// Act
		out, _ := runScript(t, "mavensettings.sh", vars, probe)

		// Assert
		assert.Equal(t, "MAVEN_ARGS=\n", out)
		assert.NoFileExists(t, vars.MavenSettings)
	})
}

func TestToolEntrypoints_mavenSettings(t *testing.T) {
	for _, name := range Names() {
		t.Run(name+" renders the Maven settings before exec on the java extra", func(t *testing.T) {
			// Act
			result := renderStage(Configs[name].Build.Stage("base", ImageInfo{Extras: []string{"java"}}))

			// Assert
			settings := strings.Index(result, "export MAVEN_ARGS=")
			execAt := strings.Index(result, "exec "+name)
			require.NotEqual(t, -1, settings)
			assert.Less(t, settings, execAt)
		})

		t.Run(name+" leaves Maven out without the java extra", func(t *testing.T) {
			// Act
			result := renderStage(Configs[name].Build.Stage("base", ImageInfo{}))

			// Assert
			assert.NotContains(t, result, "MAVEN_ARGS")
		})
	}
}
