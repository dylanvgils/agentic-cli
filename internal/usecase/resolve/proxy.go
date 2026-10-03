package resolve

import "github.com/dylanvgils/agentic-cli/internal/config"

// dindRegistryHosts are the Docker Hub endpoints image pulls need; other registries must be allowlisted explicitly.
var dindRegistryHosts = []string{"registry-1.docker.io", "auth.docker.io", "production.cloudflare.docker.com"}

// ProxyInput carries the flag-derived values ProxyMode needs.
type ProxyInput struct {
	NoProxy     bool
	MonitorFlag bool
	ProxyFlag   bool
}

// ProxyMode resolves whether the proxy is enabled and enforcing vs. monitoring. Flags win over config; --no-proxy/enabled=false always wins; monitor implies enabled.
func ProxyMode(in ProxyInput, rc *config.AgenticRC) (enabled, monitor bool) {
	if in.NoProxy {
		return false, false
	}
	if in.MonitorFlag {
		return true, true
	}
	if in.ProxyFlag {
		return true, false
	}

	if rc.Run.Proxy.Enabled != nil && !*rc.Run.Proxy.Enabled {
		return false, false
	}
	if rc.Run.Proxy.Mode == config.ModeMonitor {
		return true, true
	}

	return rc.Run.Proxy.Enabled != nil && *rc.Run.Proxy.Enabled, false
}

// ProxyAllowList merges the tool's baseline allowlist with user-configured hosts, plus Docker Hub when the Docker sidecar is on.
func ProxyAllowList(toolAllowedHosts []string, dindEnabled bool, rc *config.AgenticRC) []string {
	allow := append([]string{}, toolAllowedHosts...)
	if dindEnabled {
		allow = append(allow, dindRegistryHosts...)
	}
	return append(allow, rc.Run.Proxy.AllowedHosts...)
}
