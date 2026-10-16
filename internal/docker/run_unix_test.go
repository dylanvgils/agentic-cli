//go:build unix

package docker

import (
	"syscall"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_RunTool_signals(t *testing.T) {
	// Arrange
	client := newTestClient()
	removed := make(chan string, 2)
	stubDockerRun(t, client, func(args ...string) (string, error) {
		if args[0] == "rm" {
			removed <- args[len(args)-1]
		}
		return "", nil
	})
	var name string
	var removedWhileRunning bool
	stubRunInteractiveFunc(t, client, func(args ...string) error {
		name = toolContainerName(t, args)
		require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGHUP))
		select {
		case got := <-removed:
			removedWhileRunning = got == name
		case <-time.After(5 * time.Second):
		}
		return nil
	})

	// Act
	err := startAndRunTool(client, sandbox.RunSpec{Image: "agentic-claude"}, nil)

	// Assert
	require.NoError(t, err)
	assert.True(t, removedWhileRunning, "SIGHUP should remove the tool container while docker run still waits")
}
