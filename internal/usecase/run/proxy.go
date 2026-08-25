package run

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/housekeeping"
)

// proxyLogDir returns (creating and pruning) the host proxy access-log dir when the proxy is enabled, else an empty string.
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

// proxyRetentionDays resolves the proxy log retention window from agentic.json (a host-level setting, not .agenticrc.toml/a flag), falling back to the default when unset.
func proxyRetentionDays(toolHome string) int {
	if cfg, err := config.LoadConfig(toolHome); err == nil && cfg.ProxyLogRetentionDays > 0 {
		return cfg.ProxyLogRetentionDays
	}
	return housekeeping.DefaultProxyLogRetentionDays
}
