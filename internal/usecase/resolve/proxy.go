package resolve

import (
	"fmt"
	"slices"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/credentials"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
)

// dindRegistryHosts are the Docker Hub endpoints image pulls need.
var dindRegistryHosts = []string{"registry-1.docker.io", "auth.docker.io", "production.cloudflare.docker.com"}

// ProxyInput carries the flag-derived values ProxyMode needs.
type ProxyInput struct {
	NoProxy     bool
	MonitorFlag bool
	ProxyFlag   bool
}

// ProxyMode resolves whether the proxy is off, enforcing or monitoring; when proxyInjects, credentials turn it on and refuse an
// explicit off.
func ProxyMode(in ProxyInput, rc *config.AgenticRC, proxyInjects bool) (sandbox.ProxyMode, error) {
	mode := RequestedProxyMode(in, rc)
	if mode.Enabled() || !proxyInjects || len(rc.Run.Proxy.Credentials) == 0 {
		return mode, nil
	}

	if in.NoProxy {
		return sandbox.ProxyOff, fmt.Errorf("--no-proxy cannot be used with [[run.proxy.credentials]], which the proxy injects")
	}
	if rc.Run.Proxy.Enabled != nil && !*rc.Run.Proxy.Enabled {
		return sandbox.ProxyOff, fmt.Errorf("[run.proxy] enabled = false cannot be used with [[run.proxy.credentials]], which the proxy injects")
	}

	return sandbox.ProxyEnforce, nil
}

// ProxyAllowList merges the tool's allowlist with user hosts and credential hosts, plus Docker Hub when dind is on,
// keeping the first of any duplicates.
func ProxyAllowList(toolAllowedHosts []string, dindEnabled bool, rc *config.AgenticRC) []string {
	allow := append([]string{}, toolAllowedHosts...)
	if dindEnabled {
		allow = append(allow, dindRegistryHosts...)
	}
	allow = append(allow, rc.Run.Proxy.AllowedHosts...)
	allow = append(allow, credentials.Hosts(rc.Run.Proxy.Credentials)...)

	seen := make(map[string]bool, len(allow))
	return slices.DeleteFunc(allow, func(host string) bool {
		dup := seen[host]
		seen[host] = true
		return dup
	})
}

// RequestedProxyMode applies flags, then config, ignoring credentials. --no-proxy/enabled=false always wins; monitor implies enabled.
func RequestedProxyMode(in ProxyInput, rc *config.AgenticRC) sandbox.ProxyMode {
	if in.NoProxy {
		return sandbox.ProxyOff
	}
	if in.MonitorFlag {
		return sandbox.ProxyMonitor
	}
	if in.ProxyFlag {
		return sandbox.ProxyEnforce
	}

	if rc.Run.Proxy.Enabled != nil && !*rc.Run.Proxy.Enabled {
		return sandbox.ProxyOff
	}
	if rc.Run.Proxy.Mode == config.ModeMonitor {
		return sandbox.ProxyMonitor
	}

	if rc.Run.Proxy.Enabled != nil && *rc.Run.Proxy.Enabled {
		return sandbox.ProxyEnforce
	}

	return sandbox.ProxyOff
}
