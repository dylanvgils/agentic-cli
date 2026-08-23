package run

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/housekeeping"
	"github.com/dylanvgils/agentic-cli/internal/mount"
)

// auditPaths derives host-side watch roots from the volume list: the host
// part of every bind mount, excluding Docker named volumes.
func auditPaths(volumes []string, toolHome, containerHome string) []string {
	var paths []string
	for _, v := range volumes {
		expanded := mount.ExpandMountSpec(v, toolHome, containerHome)
		if mount.IsNamedVolume(expanded) {
			continue
		}
		paths = append(paths, filepath.Clean(mount.HostPart(expanded)))
	}
	return paths
}

// auditLogDir returns the host directory for audit logs, creating it when
// auditing is enabled and pruning any logs older than the configured
// retention window. Returns an empty string when auditing is off.
func auditLogDir(toolHome string, auditEnabled bool) (string, error) {
	if !auditEnabled {
		return "", nil
	}

	dir := filepath.Join(toolHome, "audit")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create audit log dir: %w", err)
	}

	housekeeping.PruneJSONLLogs(dir, time.Duration(auditRetentionDays(toolHome))*24*time.Hour)

	return dir, nil
}

// auditRetentionDays resolves the audit log retention window in days from
// agentic.json, falling back to the default when unset - mirrors proxyRetentionDays.
func auditRetentionDays(toolHome string) int {
	if cfg, err := config.LoadConfig(toolHome); err == nil && cfg.AuditLogRetentionDays > 0 {
		return cfg.AuditLogRetentionDays
	}
	return housekeeping.DefaultAuditLogRetentionDays
}
