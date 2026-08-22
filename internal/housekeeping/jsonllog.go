// Package housekeeping manages host-side agentic state that needs periodic
// cleanup but isn't tied to running a tool, the proxy server, or docker
// orchestration itself - e.g. pruning stale files under $AGENTIC_HOME.
package housekeeping

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DefaultProxyLogRetentionDays is how long proxy access logs are kept when no
// retention period is configured.
const DefaultProxyLogRetentionDays = 3

// DefaultAuditLogRetentionDays is how long filesystem audit logs are kept
// when no retention period is configured.
const DefaultAuditLogRetentionDays = 3

// followChanBuffer bounds how many pending lines FollowJSONL buffers before a
// slow consumer applies backpressure to the poller.
const followChanBuffer = 64

// PruneJSONLLogs removes *.jsonl log files in dir whose mtime is older than
// maxAge. maxAge <= 0 removes every log file regardless of age (used for a
// full `agentic clean` wipe). It is best-effort: a missing dir or an
// unremovable file must not fail the caller. Shared by the proxy access log
// and the filesystem audit log, which both write one JSON object per line.
func PruneJSONLLogs(dir string, maxAge time.Duration) {
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

// FollowJSONL streams each line appended to path as it's written, starting
// from the beginning of the file, until stop is called. It polls at interval
// rather than using inotify/kqueue so the same mechanism works whether the
// writer is agentic's own process (the audit watcher) or a separate
// container (the proxy sidecar); it also tolerates path not existing yet.
// lines is closed once the poller exits after stop is called.
func FollowJSONL(path string, interval time.Duration) (lines <-chan []byte, stop func()) {
	out := make(chan []byte, followChanBuffer)
	done := make(chan struct{})
	var stopOnce sync.Once

	go followJSONL(path, interval, out, done)

	return out, func() { stopOnce.Do(func() { close(done) }) }
}

// followJSONL is FollowJSONL's polling loop, run in its own goroutine.
func followJSONL(path string, interval time.Duration, out chan<- []byte, done <-chan struct{}) {
	defer close(out)

	var offset int64
	var partial []byte
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		var ok bool
		offset, partial, ok = drainJSONLLines(path, offset, partial, out, done)
		if !ok {
			return
		}

		select {
		case <-done:
			return
		case <-ticker.C:
		}
	}
}

// drainJSONLLines reads any bytes appended to path since offset, emits each
// complete line to out, and returns the advanced offset plus any trailing
// partial line (no trailing newline yet) to carry into the next call. ok is
// false when done fired while trying to send a line, signaling the caller to
// stop; a missing path or read error is not fatal - it just yields no lines
// this round, to be retried on the next tick.
func drainJSONLLines(path string, offset int64, partial []byte, out chan<- []byte, done <-chan struct{}) (newOffset int64, newPartial []byte, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return offset, partial, true
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset, partial, true
	}

	chunk, err := io.ReadAll(f)
	if err != nil || len(chunk) == 0 {
		return offset, partial, true
	}

	buf := append(partial, chunk...)
	newOffset = offset + int64(len(chunk))

	for {
		i := bytes.IndexByte(buf, '\n')
		if i < 0 {
			return newOffset, buf, true
		}

		line := append([]byte(nil), buf[:i]...)
		buf = buf[i+1:]

		select {
		case out <- line:
		case <-done:
			return newOffset, buf, false
		}
	}
}
