//go:build integration

package lifecycle

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// labelRun mirrors agentic's run label; duplicated so the tests stay black-box.
const labelRun = "agentic.run"

// teardownWithin is how long a watchdog may take to tear a killed run down.
const teardownWithin = 15 * time.Second

// runResources returns the containers and network of run id that still exist.
func runResources(t *testing.T, id string) []string {
	t.Helper()
	names := strings.Fields(platform.Docker(t, "ps", "--all", "--filter", "label="+labelRun+"="+id, "--format", "{{.Names}}"))
	if platform.DockerOK("network", "inspect", "agentic-run-"+id) {
		names = append(names, "agentic-run-"+id)
	}
	return names
}

// assertTornDown checks the watchdog removes every resource of run id.
func assertTornDown(t *testing.T, id string) {
	t.Helper()
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Empty(c, runResources(t, id), "run %s left behind", id)
	}, teardownWithin, 100*time.Millisecond)
}

// killCLI kills cmd without a chance to clean up and reaps it.
func killCLI(t *testing.T, pid int, wait func() error) {
	t.Helper()
	process, err := os.FindProcess(pid)
	require.NoError(t, err)
	require.NoError(t, process.Kill())
	_ = wait()
}
