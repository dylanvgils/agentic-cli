package precedence

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config/rc"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// dindRegistryHosts are the Docker Hub endpoints image pulls need.
var dindRegistryHosts = []string{
	"registry-1.docker.io",
	"auth.docker.io",
	// Layer blobs come from either CDN
	"production.cloudflare.docker.com",
	"production.cloudfront.docker.com",
}

// ProxyInput carries the flag-derived values ProxyMode needs.
type ProxyInput struct {
	NoProxy     bool
	MonitorFlag bool
	ProxyFlag   bool
	// Integrations are the --proxy-integration names, added to rc's
	Integrations []string
	// Backend is the --proxy-backend value; empty defers to the config
	Backend string
}

// ProxyMode resolves whether the proxy is off, enforcing or monitoring; when proxyInjects, credentials turn it on and refuse an
// explicit off.
func ProxyMode(in ProxyInput, agenticRC *rc.AgenticRC, proxyInjects bool) (sandbox.EgressMode, error) {
	mode := RequestedProxyMode(in, agenticRC)
	if mode.Enabled() || !proxyInjects || len(agenticRC.Run.Proxy.Credentials) == 0 {
		return mode, nil
	}

	if in.NoProxy {
		return sandbox.EgressOff, fmt.Errorf("--no-proxy cannot be used with [[run.proxy.credentials]], which the proxy injects")
	}
	if agenticRC.Run.Proxy.Enabled != nil && !*agenticRC.Run.Proxy.Enabled {
		return sandbox.EgressOff, fmt.Errorf("[run.proxy] enabled = false cannot be used with [[run.proxy.credentials]], which the proxy injects")
	}

	return sandbox.EgressEnforce, nil
}

// ProxyBackend returns the proxy backend --proxy-backend or rc picks, or "" for the runtime's default.
func ProxyBackend(in ProxyInput, agenticRC *rc.AgenticRC) string {
	if in.Backend != "" {
		return in.Backend
	}
	return agenticRC.Run.Proxy.Backend
}

// ProxyIntegrations returns rc's proxy integrations plus the flag's, sorted and unique.
func ProxyIntegrations(in ProxyInput, agenticRC *rc.AgenticRC) []string {
	names := slices.Concat(agenticRC.Run.Proxy.Integrations, in.Integrations)
	return slices.Compact(slices.Sorted(slices.Values(names)))
}

// ProxyRules returns the hosts proxyAllowList merges, allowed whole unless path rules name them, then rc's path rules.
// Path rules also limit a host a wildcard allows, as the proxy lets them win.
func ProxyRules(toolAllowedHosts, credentialHosts []string, dindEnabled bool, agenticRC *rc.AgenticRC) []sandbox.AllowRule {
	pathRules := mergePathRules(agenticRC.Run.Proxy.AllowedPaths)

	var rules []sandbox.AllowRule
	for _, host := range proxyAllowList(toolAllowedHosts, credentialHosts, dindEnabled, agenticRC) {
		if ruleIndex(pathRules, normalizeHost(host)) < 0 {
			rules = append(rules, sandbox.AllowRule{Host: host})
		}
	}
	return append(rules, pathRules...)
}

// RequestedProxyMode applies flags, then config, ignoring credentials. --no-proxy/enabled=false always wins; monitor implies enabled.
func RequestedProxyMode(in ProxyInput, agenticRC *rc.AgenticRC) sandbox.EgressMode {
	if in.NoProxy {
		return sandbox.EgressOff
	}
	if in.MonitorFlag {
		return sandbox.EgressMonitor
	}
	if in.ProxyFlag {
		return sandbox.EgressEnforce
	}

	if agenticRC.Run.Proxy.Enabled != nil && !*agenticRC.Run.Proxy.Enabled {
		return sandbox.EgressOff
	}
	if agenticRC.Run.Proxy.Mode == rc.ModeMonitor {
		return sandbox.EgressMonitor
	}

	if agenticRC.Run.Proxy.Enabled != nil && *agenticRC.Run.Proxy.Enabled {
		return sandbox.EgressEnforce
	}

	return sandbox.EgressOff
}

// proxyAllowList merges the tool's allowlist with user hosts and credential hosts, plus Docker Hub when dind is on,
// keeping the first of any duplicates.
func proxyAllowList(toolAllowedHosts, credentialHosts []string, dindEnabled bool, agenticRC *rc.AgenticRC) []string {
	allow := append([]string{}, toolAllowedHosts...)
	if dindEnabled {
		allow = append(allow, dindRegistryHosts...)
	}
	allow = append(allow, agenticRC.Run.Proxy.AllowedHosts...)
	allow = append(allow, credentialHosts...)

	seen := make(map[string]bool, len(allow))
	return slices.DeleteFunc(allow, func(host string) bool {
		dup := seen[host]
		seen[host] = true
		return dup
	})
}

// mergePathRules merges rules per normalized host in first-seen order, without duplicate paths.
func mergePathRules(rules []rc.RCAllowedPath) []sandbox.AllowRule {
	var merged []sandbox.AllowRule
	for _, rule := range rules {
		host := normalizeHost(rule.Host)
		i := ruleIndex(merged, host)
		if i < 0 {
			i = len(merged)
			merged = append(merged, sandbox.AllowRule{Host: host})
		}

		for _, path := range rule.Paths {
			if !slices.Contains(merged[i].Paths, path) {
				merged[i].Paths = append(merged[i].Paths, path)
			}
		}
	}
	return merged
}

// ruleIndex returns the index of host's rule in rules, or -1.
func ruleIndex(rules []sandbox.AllowRule, host string) int {
	return slices.IndexFunc(rules, func(rule sandbox.AllowRule) bool { return rule.Host == host })
}

// normalizeHost lower-cases host and drops a trailing root dot, as the proxy compares hosts.
func normalizeHost(host string) string {
	return strings.ToLower(strings.TrimSuffix(host, "."))
}
