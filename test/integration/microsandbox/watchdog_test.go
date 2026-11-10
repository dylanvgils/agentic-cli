//go:build integration

package microsandbox

import (
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWatchdog checks a killed msb run's VM is stopped by its watchdog.
func TestWatchdog(t *testing.T) {
	// Arrange
	cmd := platform.NewToolRun(t).
		InMicrosandbox().
		Launch()
	id := platform.WatchdogRunID(t, cmd.Process.Pid)
	require.Eventually(t, func() bool {
		return len(runningVMs(t, id)) > 0
	}, 2*time.Minute, 100*time.Millisecond, "VM never started")

	// Act
	require.NoError(t, cmd.Process.Kill())
	_ = cmd.Wait()

	// Assert
	assert.Eventually(t, func() bool {
		return len(runningVMs(t, id)) == 0
	}, 30*time.Second, 200*time.Millisecond, "VM outlived its run")
}
