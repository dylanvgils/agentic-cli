package platform

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenProcess(t *testing.T) {
	t.Run("waits until the process exits", func(t *testing.T) {
		// Arrange
		child := startSleeper(t)
		h, err := OpenProcess(child.Process.Pid)
		require.NoError(t, err)
		t.Cleanup(func() { _ = h.Close() })
		waited := make(chan error, 1)
		go func() { waited <- h.Wait() }()
		select {
		case err := <-waited:
			t.Fatalf("Wait returned %v while the process runs", err)
		case <-time.After(200 * time.Millisecond):
		}

		// Act
		require.NoError(t, child.Process.Kill())

		// Assert
		select {
		case err := <-waited:
			require.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Fatal("Wait didn't return after the process exited")
		}
	})

	t.Run("a process already gone is ErrProcessGone", func(t *testing.T) {
		// Arrange
		child := startSleeper(t)
		require.NoError(t, child.Process.Kill())
		_ = child.Wait()

		// Act
		_, err := OpenProcess(child.Process.Pid)

		// Assert
		assert.ErrorIs(t, err, ErrProcessGone)
	})
}

// startSleeper starts this test binary as a child that sleeps until killed.
func startSleeper(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperSleep$")
	cmd.Env = append(os.Environ(), "AGENTIC_TEST_SLEEP=1")
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	return cmd
}

// TestHelperSleep is the child startSleeper starts; it does nothing in a normal run.
func TestHelperSleep(t *testing.T) {
	if os.Getenv("AGENTIC_TEST_SLEEP") != "1" {
		t.Skip("helper process")
	}
	time.Sleep(time.Hour)
}
