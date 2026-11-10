//go:build !darwin && !windows

package platform

import "time"

// pollInterval is how often pollUntilGone checks a process.
const pollInterval = 500 * time.Millisecond

// pollUntilGone blocks until pid no longer runs.
func pollUntilGone(pid int) {
	for ProcessRunning(pid) {
		time.Sleep(pollInterval)
	}
}
