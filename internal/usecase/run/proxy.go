package run

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/housekeeping"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// proxyAllowList merges the tool's baseline allowlist with user-configured hosts.
func proxyAllowList(toolConfig tools.ToolConfig, rc *config.AgenticRC) []string {
	allow := append([]string{}, toolConfig.Runtime.AllowedHosts...)
	return append(allow, rc.Run.Proxy.AllowedHosts...)
}

// proxyLogDir returns the host directory for proxy access logs, creating it
// when the proxy is enabled and pruning any logs older than the configured
// retention window. Returns an empty string when the proxy is off.
func proxyLogDir(toolHome string, proxyEnabled bool) (string, error) {
	if !proxyEnabled {
		return "", nil
	}

	dir := filepath.Join(toolHome, "proxy")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create proxy log dir: %w", err)
	}

	housekeeping.PruneJSONLLogs(dir, time.Duration(proxyRetentionDays(toolHome))*24*time.Hour)

	return dir, nil
}

// proxyRetentionDays resolves the proxy log retention window in days from
// agentic.json, falling back to the default when unset. This is a host-level
// housekeeping setting, not a per-project or per-run one, so it does not come
// from .agenticrc.toml or a CLI flag - it's edited the same way as the other
// global settings in agentic.json (e.g. registry).
func proxyRetentionDays(toolHome string) int {
	if cfg, err := config.LoadConfig(toolHome); err == nil && cfg.ProxyLogRetentionDays > 0 {
		return cfg.ProxyLogRetentionDays
	}
	return housekeeping.DefaultProxyLogRetentionDays
}
