//go:build integration

package lifecycle

import (
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/require"
)

// TestWatchdog checks a run killed without a chance to clean up is torn down by its watchdog, without another run.
func TestWatchdog(t *testing.T) {
	t.Run("tears down a run killed while the tool runs", func(t *testing.T) {
		// Arrange
		tool := platform.NewToolRun(t).WithProxy()
		cmd, _ := tool.Start()
		id := platform.WatchdogRunID(t, cmd.Process.Pid)
		require.Len(t, runResources(t, id), 3, "expected the tool, the proxy and the run network")

		// Act
		killCLI(t, cmd.Process.Pid, cmd.Wait)

		// Assert
		assertTornDown(t, id)
	})

	t.Run("tears down a run killed during setup", func(t *testing.T) {
		// Arrange
		cmd := platform.NewToolRun(t).
			WithProxy().
			Launch()
		id := platform.WatchdogRunID(t, cmd.Process.Pid)
		require.Eventually(t, func() bool {
			return platform.DockerOK("network", "inspect", "agentic-run-"+id)
		}, time.Minute, 10*time.Millisecond, "run network never created")
		t.Logf("before the kill: %v", runResources(t, id))

		// Act
		killCLI(t, cmd.Process.Pid, cmd.Wait)

		// Assert
		assertTornDown(t, id)
	})
}
