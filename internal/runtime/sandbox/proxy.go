package sandbox

import (
	"maps"
	"slices"
	"strings"
)

// Proxy modes; ProxyOff is the zero value, so a RunSpec without one runs no proxy.
const (
	ProxyOff ProxyMode = iota
	ProxyEnforce
	ProxyMonitor
)

// The proxy's address in the tool's network, the same on every run.
const (
	ProxyHostAlias = "agentic-proxy"
	ProxyPort      = "3128"
)

// MavenSettingsPath is the Maven proxy settings file in java images.
const MavenSettingsPath = "/etc/agentic/maven/settings.xml"

// proxyIntegrationEnv maps each proxy integration to its tool env; Gradle passes GRADLE_OPTS on to its daemon.
var proxyIntegrationEnv = map[string][]string{
	"gradle": {"GRADLE_OPTS=" + strings.Join([]string{
		"-Dhttp.proxyHost=" + ProxyHostAlias,
		"-Dhttp.proxyPort=" + ProxyPort,
		"-Dhttps.proxyHost=" + ProxyHostAlias,
		"-Dhttps.proxyPort=" + ProxyPort,
	}, " ")},
	"maven": {"MAVEN_ARGS=--global-settings " + MavenSettingsPath},
}

// ProxyMode selects whether the egress proxy runs and whether it blocks or only logs disallowed hosts.
type ProxyMode int

// AllowRule allows Host on every path, or with Paths only on those, which the proxy decrypts the host's traffic to
// check; Host is exact or a "."/"*." wildcard, exact when it has Paths.
type AllowRule struct {
	Host  string
	Paths []string
}

// ProxySpec configures the egress proxy sidecar; unless Mode is ProxyOff, the tool reaches out only through it.
type ProxySpec struct {
	Mode    ProxyMode
	Rules   []AllowRule // merged allowlist: tool baseline, user hosts and path rules, credential hosts
	LogFile string      // host path of this run's JSON-lines access log
	// Integrations name tools to route through the proxy, e.g. "maven"
	Integrations []string
}

// Enabled reports whether the proxy runs, in either mode.
func (m ProxyMode) Enabled() bool {
	return m != ProxyOff
}

// HasPathRules reports whether any rule allows only some paths.
func (s ProxySpec) HasPathRules() bool {
	return slices.ContainsFunc(s.Rules, func(r AllowRule) bool { return len(r.Paths) > 0 })
}

// AllowWhole returns rules with each one allowing every path, for a runtime that can't match paths.
func AllowWhole(rules []AllowRule) []AllowRule {
	whole := make([]AllowRule, len(rules))
	for i, rule := range rules {
		whole[i] = AllowRule{Host: rule.Host}
	}
	return whole
}

// WholeHosts returns the hosts of the rules that allow every path.
func (s ProxySpec) WholeHosts() []string {
	var hosts []string
	for _, rule := range s.Rules {
		if len(rule.Paths) == 0 {
			hosts = append(hosts, rule.Host)
		}
	}
	return hosts
}

// ProxyIntegrations returns the known proxy integration names, sorted.
func ProxyIntegrations() []string {
	return slices.Sorted(maps.Keys(proxyIntegrationEnv))
}

// IsProxyIntegration reports whether name is a known proxy integration.
func IsProxyIntegration(name string) bool {
	_, ok := proxyIntegrationEnv[name]
	return ok
}

// ProxyIntegrationEnv returns the tool env vars the named integrations set.
func ProxyIntegrationEnv(names []string) []string {
	var env []string
	for _, name := range names {
		env = append(env, proxyIntegrationEnv[name]...)
	}
	return env
}

// ProxyIntegrationEnvNames returns the env var names the named integrations set.
func ProxyIntegrationEnvNames(names []string) []string {
	env := ProxyIntegrationEnv(names)
	keys := make([]string, len(env))
	for i, entry := range env {
		keys[i], _, _ = strings.Cut(entry, "=")
	}
	return keys
}
