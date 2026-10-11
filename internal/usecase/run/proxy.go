package run

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/housekeeping"
)

// proxyLogDir returns the host log dir when the proxy is enabled, else an empty string.
func proxyLogDir(toolHome string, proxyEnabled bool) string {
	if !proxyEnabled {
		return ""
	}
	return filepath.Join(toolHome, config.LogsDirName)
}

// prepareProxyLogDir creates dir and prunes the proxy logs in it older than toolHome's retention window.
func prepareProxyLogDir(dir, toolHome string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	housekeeping.PruneProxyLogs(dir, time.Duration(proxyRetentionDays(toolHome))*24*time.Hour)
	return nil
}

// proxyRetentionDays resolves the proxy log retention window from agentic.json (a host-level setting, not .agenticrc.toml/a flag), falling back to the default when unset.
func proxyRetentionDays(toolHome string) int {
	if cfg, err := config.LoadConfig(toolHome); err == nil && cfg.ProxyLogRetentionDays > 0 {
		return cfg.ProxyLogRetentionDays
	}
	return housekeeping.DefaultProxyLogRetentionDays
}
