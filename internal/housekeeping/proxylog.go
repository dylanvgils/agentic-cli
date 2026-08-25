// Package housekeeping prunes stale host-side agentic state under $AGENTIC_HOME that isn't tied to a specific tool run.
package housekeeping

import (
	"os"
	"path/filepath"
	"time"
)

// DefaultProxyLogRetentionDays is how long proxy access logs are kept when no retention period is configured.
const DefaultProxyLogRetentionDays = 3

// PruneProxyLogs removes *.jsonl access-log files in dir older than maxAge (maxAge <= 0 removes all). Best-effort: never fails the caller.
func PruneProxyLogs(dir string, maxAge time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}

		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}
}
