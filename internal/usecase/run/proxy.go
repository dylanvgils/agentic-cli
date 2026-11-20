package run

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
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

// proxyLogFile returns rs's access-log path in its run log dir, started at start, when the proxy is enabled, else "".
func proxyLogFile(rs sandbox.RunSpec, start time.Time) string {
	if !rs.Egress.Mode.Enabled() {
		return ""
	}
	return filepath.Join(logging.Dir(rs.ToolHome, rs.RunID.String(), start), proxy.LogFileName)
}

// prepareProxyLogDir creates runDir and prunes the log dirs older than agentic.json's retention, the default if unreadable.
func prepareProxyLogDir(runDir, toolHome string) error {
	if err := os.MkdirAll(runDir, 0o750); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	cfg, err := config.LoadConfig(toolHome)
	if err != nil {
		cfg = &config.CliConfig{}
	}
	logging.Prune(toolHome, cfg.LogRetention())
	return nil
}
