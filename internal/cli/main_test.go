package cli

import (
	"os"
	"testing"
)

// TestMain replaces the watchdog for every test, since the real one reruns this test binary.
func TestMain(m *testing.M) {
	watchdogs = &fakeWatchdogs{}
	os.Exit(m.Run())
}
