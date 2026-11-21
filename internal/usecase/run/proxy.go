package run

import (
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
)

// proxyRules resolves the proxy's allow rules, allowing path rule hosts whole on a runtime that can't match paths.
func proxyRules(c runConfig) []sandbox.AllowRule {
	rules := resolve.ProxyRules(c.Tool.Runtime.AllowedHosts, c.Dind.Enabled, c.RC)
	if c.Runtime.Caps.PathRules {
		return rules
	}
	return sandbox.AllowWhole(rules)
}

// proxyIntegrations returns the flag's and rc's proxy integrations when the proxy runs and the runtime supports them.
func proxyIntegrations(flags resolve.ProxyInput, rc *config.AgenticRC, mode sandbox.EgressMode, features sandbox.Features) []string {
	if !mode.Enabled() || !features.ProxyIntegrations {
		return nil
	}
	return resolve.ProxyIntegrations(flags, rc)
}

// proxyLogFile returns rs's access-log path in logDir when the proxy is enabled, else "".
func proxyLogFile(rs sandbox.RunSpec, logDir string) string {
	if !rs.Egress.Mode.Enabled() {
		return ""
	}
	return filepath.Join(logDir, proxy.LogFileName)
}
