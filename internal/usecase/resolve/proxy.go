package resolve

import (
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
)

// dindRegistryHosts are the Docker Hub endpoints image pulls need.
var dindRegistryHosts = []string{"registry-1.docker.io", "auth.docker.io", "production.cloudflare.docker.com"}

// ProxyInput carries the flag-derived values ProxyMode needs.
type ProxyInput struct {
	NoProxy     bool
	MonitorFlag bool
	ProxyFlag   bool
}

// ProxyMode resolves whether the proxy is off, enforcing or monitoring. Flags win over config; --no-proxy/enabled=false always wins; monitor implies enabled.
func ProxyMode(in ProxyInput, rc *config.AgenticRC) docker.ProxyMode {
	if in.NoProxy {
		return docker.ProxyOff
	}
	if in.MonitorFlag {
		return docker.ProxyMonitor
	}
	if in.ProxyFlag {
		return docker.ProxyEnforce
	}

	if rc.Run.Proxy.Enabled != nil && !*rc.Run.Proxy.Enabled {
		return docker.ProxyOff
	}
	if rc.Run.Proxy.Mode == config.ModeMonitor {
		return docker.ProxyMonitor
	}

	if rc.Run.Proxy.Enabled != nil && *rc.Run.Proxy.Enabled {
		return docker.ProxyEnforce
	}

	return docker.ProxyOff
}

// ProxyAllowList merges the tool's allowlist with user hosts, plus Docker Hub when dind is on.
func ProxyAllowList(toolAllowedHosts []string, dindEnabled bool, rc *config.AgenticRC) []string {
	allow := append([]string{}, toolAllowedHosts...)
	if dindEnabled {
		allow = append(allow, dindRegistryHosts...)
	}
	return append(allow, rc.Run.Proxy.AllowedHosts...)
}
