package tools

import (
	"encoding/xml"
	"os"
	"os/exec"
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
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	dir := t.TempDir()
	vars := defaultScriptVars()
	vars.MavenSettingsTemplate = filepath.Join(dir, "template.xml")
	vars.MavenSettings = filepath.Join(dir, "settings.xml")
	require.NoError(t, os.WriteFile(vars.MavenSettingsTemplate, []byte("<port>"+mavenPortPlaceholder+"</port>\n"), 0o644))
	script := strings.Join(renderScript("mavensettings.sh", vars), "\n") + "\necho \"MAVEN_ARGS=${MAVEN_ARGS:-}\""

	t.Run("renders the port and puts the settings before the user's args", func(t *testing.T) {
		// Arrange
		cmd := exec.Command("bash", "-euo", "pipefail", "-c", script)
		cmd.Env = []string{sandbox.MavenProxyPortEnv + "=3128", "MAVEN_ARGS=-Dtest=1"}

		// Act
		out, err := cmd.Output()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "MAVEN_ARGS=--global-settings "+vars.MavenSettings+" -Dtest=1\n", string(out))
		got, err := os.ReadFile(vars.MavenSettings)
		require.NoError(t, err)
		assert.Equal(t, "<port>3128</port>\n", string(got))
	})

	t.Run("without the port changes nothing", func(t *testing.T) {
		// Arrange
		require.NoError(t, os.RemoveAll(vars.MavenSettings))
		cmd := exec.Command("bash", "-euo", "pipefail", "-c", script)
		cmd.Env = []string{}

		// Act
		out, err := cmd.Output()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "MAVEN_ARGS=\n", string(out))
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
