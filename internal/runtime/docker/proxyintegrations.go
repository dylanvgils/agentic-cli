package docker

import (
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
)

// proxyIntegrationEnv maps each egress integration to the tool env routing it through the proxy sidecar; Gradle passes
// GRADLE_OPTS on to its daemon.
var proxyIntegrationEnv = map[string][]string{
	"gradle": {"GRADLE_OPTS=" + strings.Join([]string{
		"-Dhttp.proxyHost=" + tools.ProxyHostAlias,
		"-Dhttp.proxyPort=" + tools.ProxyPort,
		"-Dhttps.proxyHost=" + tools.ProxyHostAlias,
		"-Dhttps.proxyPort=" + tools.ProxyPort,
	}, " ")},
	"maven": {"MAVEN_ARGS=--global-settings " + tools.MavenSettingsPath},
}

// ProxyIntegrationEnvNames returns the env var names the named integrations set in the tool.
func ProxyIntegrationEnvNames(names []string) []string {
	env := proxyIntegrationToolEnv(names)
	keys := make([]string, len(env))
	for i, entry := range env {
		keys[i], _, _ = strings.Cut(entry, "=")
	}
	return keys
}

// proxyIntegrationToolEnv returns the tool env vars the named integrations set.
func proxyIntegrationToolEnv(names []string) []string {
	var env []string
	for _, name := range names {
		env = append(env, proxyIntegrationEnv[name]...)
	}
	return env
}
