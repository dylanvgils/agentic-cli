//go:build integration

package platform

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// WatchdogRunID returns the run ID of the watchdog guarding agentic process pid, read from its args.
func WatchdogRunID(t *testing.T, pid int) string {
	t.Helper()
	pidArg := "--pid=" + strconv.Itoa(pid)
	var id string
	require.Eventually(t, func() bool {
		id = findWatchdogRun(pidArg)
		return id != ""
	}, time.Minute, 20*time.Millisecond, "no watchdog for pid %d", pid)
	return id
}

// findWatchdogRun returns the --run arg of the watchdog whose args hold pidArg, or "".
func findWatchdogRun(pidArg string) string {
	cmdlines, _ := filepath.Glob("/proc/[0-9]*/cmdline")
	for _, path := range cmdlines {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		args := strings.Split(string(data), "\x00")
		if !slices.Contains(args, "__watch") || !slices.Contains(args, pidArg) {
			continue
		}
		for _, a := range args {
			if id, ok := strings.CutPrefix(a, "--run="); ok {
				return id
			}
		}
	}
	return ""
}
