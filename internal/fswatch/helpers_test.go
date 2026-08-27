package fswatch

import (
	"bufio"
	"bytes"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// syncBuffer is a bytes.Buffer safe for concurrent writer/reader goroutines in tests.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) Entries(t *testing.T) []Entry {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()

	var entries []Entry
	scanner := bufio.NewScanner(bytes.NewReader(s.buf.Bytes()))
	for scanner.Scan() {
		var entry Entry
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &entry))
		entries = append(entries, entry)
	}
	return entries
}

// countMatching returns how many entries satisfy match.
func countMatching(entries []Entry, match func(Entry) bool) int {
	n := 0
	for _, entry := range entries {
		if match(entry) {
			n++
		}
	}
	return n
}

// waitForEntry polls until match returns true for some logged entry, or fails after timeout.
func waitForEntry(t *testing.T, buf *syncBuffer, timeout time.Duration, match func(Entry) bool) Entry {
	t.Helper()
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		for _, entry := range buf.Entries(t) {
			if match(entry) {
				return entry
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for matching entry; got: %+v", buf.Entries(t))
	return Entry{}
}
