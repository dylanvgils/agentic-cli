package run

import (
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/config/agenticrc"
	"github.com/dylanvgils/agentic-cli/internal/config/precedence"
	"github.com/dylanvgils/agentic-cli/internal/network/credentials"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// proxyRules resolves the proxy's allow rules, allowing path rule hosts whole on a runtime that can't match paths.
func proxyRules(c runConfig) []sandbox.AllowRule {
	rules := precedence.ProxyRules(c.Tool.Runtime.AllowedHosts, credentials.Hosts(c.RC.Run.Proxy.Credentials), c.Dind.Enabled, c.RC)
	if c.Runtime.Caps.PathRules {
		return rules
	}
	return sandbox.AllowWhole(rules)
}

// proxyIntegrations returns the flag's and rc's proxy integrations when the proxy runs and the runtime supports them.
func proxyIntegrations(flags precedence.ProxyInput, rc *agenticrc.Config, mode sandbox.EgressMode, features sandbox.Features) []string {
	if !mode.Enabled() || !features.ProxyIntegrations {
		return nil
	}
	return precedence.ProxyIntegrations(flags, rc)
}

// proxyLogFile returns rs's access-log path in logDir when the proxy is enabled, else "".
func proxyLogFile(rs sandbox.RunSpec, logDir string) string {
	if !rs.Egress.Mode.Enabled() {
		return ""
	}
	return filepath.Join(logDir, proxy.LogFileName)
}
