// Package housekeeping prunes stale host-side agentic state under $AGENTIC_HOME that isn't tied to a specific tool run.
package housekeeping

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DefaultProxyLogRetentionDays is how long proxy access logs are kept when no retention period is configured.
const DefaultProxyLogRetentionDays = 3

// DefaultAuditLogRetentionDays is how long filesystem audit logs are kept when no retention period is configured.
const DefaultAuditLogRetentionDays = 3

// followChanBuffer bounds how many pending lines FollowJSONL buffers before backpressure applies.
const followChanBuffer = 64

// PruneJSONLLogs removes *.jsonl files in dir matching prefix ("" matches all) older than maxAge (maxAge <= 0 removes all). Best-effort: never fails the caller.
func PruneJSONLLogs(dir, prefix string, maxAge time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}

		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}
}

// FollowJSONL streams each line appended to path until stop is called, then closes lines. Polls, since the writer may be a separate container.
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

// drainJSONLLines emits complete new lines to out and returns the advanced offset plus any trailing partial line.
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
