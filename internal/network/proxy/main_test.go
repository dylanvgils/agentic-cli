package proxy

import (
	"log/slog"
	"os"
	"testing"
)

// TestMain keeps the proxy's log records out of test output unless a test stubs slog.
func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.DiscardHandler))
	os.Exit(m.Run())
}
