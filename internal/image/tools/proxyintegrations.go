package tools

import (
	"strings"
)

const (
	// MavenProxyHostEnv, MavenProxyUserEnv and MavenProxyPasswordEnv carry the proxy endpoint into the Maven settings,
	// which read them as ${env.NAME}; the entrypoint writes MavenProxyPortEnv in, as Maven can't read a port that way.
	MavenProxyHostEnv     = "AGENTIC_MAVEN_PROXY_HOST"
	MavenProxyPortEnv     = "AGENTIC_MAVEN_PROXY_PORT"
	MavenProxyUserEnv     = "AGENTIC_MAVEN_PROXY_USER"
	MavenProxyPasswordEnv = "AGENTIC_MAVEN_PROXY_PASSWORD"
)

// ProxyEndpoint is the proxy as a tool reaches it; User and Password are empty when it takes no auth.
type ProxyEndpoint struct {
	Host     string
	Port     string
	User     string
	Password string
}

// proxyIntegrationEnv maps each egress integration to the tool env routing it through the proxy at an endpoint.
var proxyIntegrationEnv = map[string]func(ProxyEndpoint) []string{
	"gradle": gradleProxyEnv,
	"maven":  mavenProxyEnv,
}

// ProxyIntegrationEnv returns the tool env vars the named integrations set for the proxy at ep.
func ProxyIntegrationEnv(names []string, ep ProxyEndpoint) []string {
	var env []string
	for _, name := range names {
		if envFor, ok := proxyIntegrationEnv[name]; ok {
			env = append(env, envFor(ep)...)
		}
	}
	return env
}

// ProxyIntegrationEnvNames returns the env var names the named integrations set in the tool.
func ProxyIntegrationEnvNames(names []string) []string {
	env := ProxyIntegrationEnv(names, ProxyEndpoint{})
	keys := make([]string, len(env))
	for i, entry := range env {
		keys[i], _, _ = strings.Cut(entry, "=")
	}
	return keys
}

// gradleProxyEnv sets GRADLE_OPTS, which Gradle passes on to its daemon. With auth, it also re-enables Basic auth for
// HTTPS tunnels, which the JDK turns off by default and the Gradle wrapper's download needs.
func gradleProxyEnv(ep ProxyEndpoint) []string {
	var opts []string
	for _, scheme := range []string{"http", "https"} {
		opts = append(opts,
			"-D"+scheme+".proxyHost="+ep.Host,
			"-D"+scheme+".proxyPort="+ep.Port,
		)
	}
	if ep.User != "" {
		for _, scheme := range []string{"http", "https"} {
			opts = append(opts,
				"-D"+scheme+".proxyUser="+ep.User,
				"-D"+scheme+".proxyPassword="+ep.Password,
			)
		}
		opts = append(opts, "-Djdk.http.auth.tunneling.disabledSchemes=")
	}
	return []string{"GRADLE_OPTS=" + strings.Join(opts, " ")}
}

// mavenProxyEnv points Maven at the settings the entrypoint renders and gives them the endpoint.
func mavenProxyEnv(ep ProxyEndpoint) []string {
	return []string{
		"MAVEN_ARGS=--global-settings " + mavenSettingsPath,
		MavenProxyHostEnv + "=" + ep.Host,
		MavenProxyPortEnv + "=" + ep.Port,
		MavenProxyUserEnv + "=" + ep.User,
		MavenProxyPasswordEnv + "=" + ep.Password,
	}
}
