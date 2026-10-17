//go:build integration

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClosedTerminal checks a run whose terminal closes removes its tool container and sidecars instead of leaving the
// tool running unattended.
func TestClosedTerminal(t *testing.T) {
	// Arrange
	dir := proxyWorkDir(t, "example.test")
	ready := filepath.Join(dir, ".ready")
	cmd := agenticCmdIn(dir, "run", "--trust-dir", "--proxy", testTool, "--", "sh", "-c", "touch .ready; while true; do sleep 1; done")
	require.NoError(t, cmd.Start())
	require.Eventually(t, func() bool {
		_, err := os.Stat(ready)
		return err == nil
	}, 2*time.Minute, 100*time.Millisecond, "tool never started")
	running := runContainers(t)
	require.Len(t, running, 2, "expected the tool container and the proxy")

	// Act
	require.NoError(t, cmd.Process.Signal(syscall.SIGHUP))
	// Only the exit matters; its status after SIGHUP doesn't
	waited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(waited)
	}()

	// Assert
	select {
	case <-waited:
	case <-time.After(time.Minute):
		_ = cmd.Process.Kill()
		t.Fatal("agentic did not exit after SIGHUP")
	}
	assert.Empty(t, runContainers(t))
}

// runContainers returns the names of the agentic containers that exist, running or not.
func runContainers(t *testing.T) []string {
	t.Helper()
	out, err := exec.Command("docker", "ps", "--all", "--filter", "label=project=agentic-cli", "--format", "{{.Names}}").Output()
	require.NoError(t, err)
	return strings.Fields(string(out))
}
