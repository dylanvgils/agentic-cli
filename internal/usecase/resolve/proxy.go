package resolve

import "github.com/dylanvgils/agentic-cli/internal/config"

// ProxyInput carries the flag-derived values ProxyMode needs.
type ProxyInput struct {
	NoProxy     bool
	MonitorFlag bool
	ProxyFlag   bool
}

// ProxyMode determines whether the egress proxy is on for this run, and if
// so, whether it enforces the allowlist or only monitors it. Flags win over
// config; an explicit "off" (--no-proxy, or enabled = false) always beats
// mode, since mode only matters once the proxy is otherwise on. Monitor mode
// (flag or config) implies the proxy is enabled.
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

// ProxyAllowList merges the tool's baseline allowlist with user-configured hosts.
func ProxyAllowList(toolAllowedHosts []string, rc *config.AgenticRC) []string {
	allow := append([]string{}, toolAllowedHosts...)
	return append(allow, rc.Run.Proxy.AllowedHosts...)
}
