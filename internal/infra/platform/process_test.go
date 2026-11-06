package platform

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessRunning(t *testing.T) {
	t.Run("the current process is running", func(t *testing.T) {
		// Act
		running := ProcessRunning(os.Getpid())

		// Assert
		assert.True(t, running)
	})

	t.Run("an exited process isn't", func(t *testing.T) {
		// Arrange
		cmd := exec.Command(os.Args[0], "-test.run=^$")
		require.NoError(t, cmd.Run())

		// Act
		running := ProcessRunning(cmd.Process.Pid)

		// Assert
		assert.False(t, running)
	})
}
