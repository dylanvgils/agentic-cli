package sandbox

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Maven proxy endpoint env, read by the image's Maven settings.
const (
	MavenProxyHostEnv     = "AGENTIC_MAVEN_PROXY_HOST"
	MavenProxyPortEnv     = "AGENTIC_MAVEN_PROXY_PORT"
	MavenProxyUserEnv     = "AGENTIC_MAVEN_PROXY_USER"
	MavenProxyPasswordEnv = "AGENTIC_MAVEN_PROXY_PASSWORD"
)

// ProxyEndpoint is the proxy as a tool reaches it; no User means no auth.
type ProxyEndpoint struct {
	Host     string
	Port     string
	User     string
	Password string
}

// integrationEnv maps each egress integration to its tool env.
var integrationEnv = map[string]func(ProxyEndpoint) []string{
	"gradle": gradleEnv,
	"maven":  mavenEnv,
}

// EgressIntegrations returns the known egress integration names, sorted.
func EgressIntegrations() []string {
	return slices.Sorted(maps.Keys(integrationEnv))
}

// CheckEgressIntegrations refuses the first name that isn't a known egress integration.
func CheckEgressIntegrations(names []string) error {
	for _, name := range names {
		if _, ok := integrationEnv[name]; !ok {
			return fmt.Errorf("unknown integration %q: must be one of %s", name, strings.Join(EgressIntegrations(), ", "))
		}
	}
	return nil
}

// IntegrationEnv returns the env the named integrations set for the proxy at ep.
func IntegrationEnv(names []string, ep ProxyEndpoint) []string {
	var env []string
	for _, name := range names {
		if envFor, ok := integrationEnv[name]; ok {
			env = append(env, envFor(ep)...)
		}
	}
	return env
}

// IntegrationEnvNames returns the env var names the named integrations set.
func IntegrationEnvNames(names []string) []string {
	env := IntegrationEnv(names, ProxyEndpoint{})
	keys := make([]string, len(env))
	for i, entry := range env {
		keys[i], _, _ = strings.Cut(entry, "=")
	}
	return keys
}

// gradleEnv sets GRADLE_OPTS; with auth it re-enables Basic for tunnels, which the JDK turns off.
func gradleEnv(ep ProxyEndpoint) []string {
	var opts []string
	for _, scheme := range []string{"http", "https"} {
		opts = append(opts,
			"-D"+scheme+".proxyHost="+ep.Host,
			"-D"+scheme+".proxyPort="+ep.Port,
		)
		if ep.User != "" {
			opts = append(opts,
				"-D"+scheme+".proxyUser="+ep.User,
				"-D"+scheme+".proxyPassword="+ep.Password,
			)
		}
	}
	if ep.User != "" {
		opts = append(opts, "-Djdk.http.auth.tunneling.disabledSchemes=")
	}
	return []string{"GRADLE_OPTS=" + strings.Join(opts, " ")}
}

// mavenEnv passes the endpoint to the image's Maven settings.
func mavenEnv(ep ProxyEndpoint) []string {
	return []string{
		MavenProxyHostEnv + "=" + ep.Host,
		MavenProxyPortEnv + "=" + ep.Port,
		MavenProxyUserEnv + "=" + ep.User,
		MavenProxyPasswordEnv + "=" + ep.Password,
	}
}
