// Package housekeeping manages host-side agentic state that needs periodic
// cleanup but isn't tied to running a tool, the proxy server, or docker
// orchestration itself - e.g. pruning stale files under $AGENTIC_HOME.
package housekeeping

import (
	"os"
	"path/filepath"
	"time"
)

// DefaultProxyLogRetentionDays is how long proxy access logs are kept when no
// retention period is configured.
const DefaultProxyLogRetentionDays = 3

// PruneProxyLogs removes *.jsonl access-log files in dir whose mtime is older
// than maxAge. maxAge <= 0 removes every log file regardless of age (used for
// a full `agentic clean` wipe). It is best-effort: a missing dir or an
// unremovable file must not fail the caller.
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
