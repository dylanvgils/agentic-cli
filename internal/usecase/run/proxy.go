package run

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
)

// defaultProxyLogRetentionDays is how long proxy access logs are kept when no retention period is configured.
const defaultProxyLogRetentionDays = 3

// proxyRules resolves the proxy's allow rules, allowing path rule hosts whole on a runtime that can't match paths.
func proxyRules(toolConfig tools.ToolConfig, in Input, rc *config.AgenticRC, caps sandbox.Capabilities) []sandbox.AllowRule {
	rules := resolve.ProxyRules(toolConfig.Runtime.AllowedHosts, in.DindEnabled, rc)
	if caps.PathRules {
		return rules
	}
	return sandbox.AllowWhole(rules)
}

// proxyLogFile returns a new access-log path in the tool home's logs dir when the proxy is enabled, else an empty string.
func proxyLogFile(toolHome string, proxyEnabled bool) (string, error) {
	if !proxyEnabled {
		return "", nil
	}

	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("name proxy log: %w", err)
	}
	return filepath.Join(toolHome, config.LogsDirName, proxy.LogFilePrefix+hex.EncodeToString(buf)+".jsonl"), nil
}

// prepareProxyLogDir creates dir and prunes the proxy logs in it older than toolHome's retention window.
func prepareProxyLogDir(dir, toolHome string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	proxy.PruneLogs(dir, time.Duration(proxyRetentionDays(toolHome))*24*time.Hour)
	return nil
}

// proxyRetentionDays resolves the proxy log retention window from agentic.json (a host-level setting, not .agenticrc.toml/a flag), falling back to the default when unset.
func proxyRetentionDays(toolHome string) int {
	if cfg, err := config.LoadConfig(toolHome); err == nil && cfg.ProxyLogRetentionDays > 0 {
		return cfg.ProxyLogRetentionDays
	}
	return defaultProxyLogRetentionDays
}
