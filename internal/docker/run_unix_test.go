//go:build unix

package docker

import (
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Client_attachTool_signals(t *testing.T) {
	// Arrange
	client := newTestClient()
	removed := make(chan string, 2)
	stubDockerRun(t, client, func(args ...string) (string, error) {
		removed <- args[len(args)-1]
		return "", nil
	})
	var removedWhileRunning string
	stubRunInteractiveFunc(t, client, func(...string) error {
		require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGHUP))
		select {
		case removedWhileRunning = <-removed:
		case <-time.After(5 * time.Second):
		}
		return nil
	})

	// Act
	err := client.attachTool("agentic-claude-test", []string{"run"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "agentic-claude-test", removedWhileRunning, "SIGHUP should remove the tool container while docker run still waits")
}
