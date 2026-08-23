package docker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/fswatch"
)

// auditHandle owns the host-side watcher and its per-run log file for one run.
type auditHandle struct {
	logPath string
	watcher *fswatch.Watcher
	logFile *os.File
}

// newAuditHandle derives the per-run log path and constructs (but does not Start) the watcher.
func newAuditHandle(rs RunSpec) (auditHandle, error) {
	id, err := randID()
	if err != nil {
		return auditHandle{}, err
	}

	logPath := filepath.Join(rs.AuditLogDir, id+".jsonl")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return auditHandle{}, fmt.Errorf("open audit log: %w", err)
	}

	watcher := fswatch.New(rs.AuditPaths, fswatch.NewLogger(f), fswatch.Options{Exclude: rs.AuditExclude})
	return auditHandle{logPath: logPath, watcher: watcher, logFile: f}, nil
}

// Stop stops the watcher and closes the log file. Idempotent, safe to defer.
func (h auditHandle) Stop() {
	if h.watcher != nil {
		h.watcher.Stop()
	}
	if h.logFile != nil {
		_ = h.logFile.Close()
	}
}

// PrintSummary reports what the audit log observed, printed only when there is something to report.
func (h auditHandle) PrintSummary(w io.Writer) {
	counts, total, warnings := h.summarize()
	if total == 0 && warnings == 0 {
		return
	}

	fmt.Fprintf(w, "\nagentic audit observed %d open(s), %d write(s), %d create(s), %d delete(s), %d rename(s) under %s",
		counts[fswatch.OpOpen], counts[fswatch.OpWrite], counts[fswatch.OpCreate], counts[fswatch.OpDelete], counts[fswatch.OpRename], h.logPath)
	if warnings > 0 {
		fmt.Fprintf(w, ", %d warning(s) - see %s", warnings, h.logPath)
	}
	fmt.Fprintln(w)
}

// summarize reads the audit log and tallies entries per Op, plus a count of
// Detail-only meta entries (warnings) that aren't tied to any Op.
func (h auditHandle) summarize() (counts map[fswatch.Op]int, total, warnings int) {
	f, err := os.Open(h.logPath)
	if err != nil {
		return nil, 0, 0
	}
	defer func() { _ = f.Close() }()

	counts = make(map[fswatch.Op]int)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry fswatch.Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.Op == "" {
			warnings++
			continue
		}
		counts[entry.Op]++
		total++
	}
	_ = scanner.Err()

	return counts, total, warnings
}

// setupAudit starts the audit watcher when enabled, returning a cleanup func to defer. No-op if disabled or a dry run.
func setupAudit(rs RunSpec) (cleanup func(), err error) {
	if !rs.AuditEnabled || rs.DryRun {
		return func() {}, nil
	}

	handle, err := newAuditHandle(rs)
	if err != nil {
		return nil, err
	}
	if err := handle.watcher.Start(); err != nil {
		handle.Stop()
		return nil, err
	}

	// Ensure the watcher is stopped even on Ctrl-C, mirroring setupProxy.
	stop := guardSignals()

	cleanup = func() {
		handle.Stop()
		stop()
		handle.PrintSummary(os.Stderr)
	}
	return cleanup, nil
}
