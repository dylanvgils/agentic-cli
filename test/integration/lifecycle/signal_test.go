//go:build integration

package lifecycle

import (
	"syscall"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClosedTerminal checks a run whose terminal closes removes its tool container and sidecars instead of leaving the
// tool running unattended.
func TestClosedTerminal(t *testing.T) {
	// Arrange
	tool := platform.NewToolRun(t).
		Allowing("example.test").
		WithProxy()
	cmd, _ := tool.Start()
	running := tool.Containers()
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
	for _, name := range running {
		assert.False(t, platform.DockerOK("container", "inspect", name), "container %s left behind", name)
	}
}
