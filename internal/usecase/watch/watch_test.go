package watch

import (
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRunID is the run ID of test runs.
const testRunID sandbox.RunID = "0123456789ab"

func TestService_Watch(t *testing.T) {
	t.Run("tears the run down once after the process exits", func(t *testing.T) {
		// Arrange
		out := iostreamstest.StubOut(t)
		toolHome := t.TempDir()
		runs := &fakeRuns{}
		child := startChild(t)
		start := time.Now()

		// Act
		err := newTestService(runs).Watch(Target{
			RunRef: sandbox.RunRef{
				ID:       testRunID,
				Runtime:  sandbox.RuntimeDocker,
				Tool:     "claude",
				ToolHome: toolHome,
			},
			PID: child.Process.Pid,
		})

		// Assert
		require.NoError(t, err)
		assert.GreaterOrEqual(t, time.Since(start), childLifetime/2, "waits for the exit")
		assert.Equal(t, ReadyLine+"\n", out.String())
		run := sandbox.RunRef{
			ID:       testRunID,
			Runtime:  sandbox.RuntimeDocker,
			Tool:     "claude",
			ToolHome: toolHome,
		}
		assert.Equal(t, []sandbox.RunRef{run}, runs.calls)
	})

	t.Run("a process already gone is reaped", func(t *testing.T) {
		// Arrange
		out := iostreamstest.StubOut(t)
		runs := &fakeRuns{}
		gone := exec.Command(os.Args[0], "-test.run=^$")
		require.NoError(t, gone.Run())

		// Act
		err := newTestService(runs).Watch(Target{
			RunRef: sandbox.RunRef{
				ID:       testRunID,
				Runtime:  sandbox.RuntimeMicrosandbox,
				Tool:     "claude",
				ToolHome: t.TempDir(),
			},
			PID: gone.Process.Pid,
		})

		// Assert
		require.NoError(t, err)
		assert.Empty(t, out.String(), "nobody waits for it to be ready")
		assert.Len(t, runs.calls, 1)
	})

	t.Run("returns the teardown's error", func(t *testing.T) {
		// Arrange
		iostreamstest.StubOut(t)
		toolHome := t.TempDir()
		runs := &fakeRuns{err: errors.New("stub: teardown failed")}
		gone := exec.Command(os.Args[0], "-test.run=^$")
		require.NoError(t, gone.Run())

		// Act
		err := newTestService(runs).Watch(Target{
			RunRef: sandbox.RunRef{
				ID:       testRunID,
				Runtime:  sandbox.RuntimeDocker,
				Tool:     "claude",
				ToolHome: toolHome,
			},
			PID: gone.Process.Pid,
		})

		// Assert
		require.EqualError(t, err, "stub: teardown failed")
	})

	t.Run("waits the grace before the teardown", func(t *testing.T) {
		// Arrange
		iostreamstest.StubOut(t)
		const grace = 100 * time.Millisecond
		runs := &fakeRuns{}
		svc := newTestService(runs)
		svc.reapGrace = grace
		gone := exec.Command(os.Args[0], "-test.run=^$")
		require.NoError(t, gone.Run())
		start := time.Now()

		// Act
		err := svc.Watch(Target{
			RunRef: sandbox.RunRef{
				ID:       testRunID,
				Runtime:  sandbox.RuntimeDocker,
				ToolHome: t.TempDir(),
			},
			PID: gone.Process.Pid,
		})

		// Assert
		require.NoError(t, err)
		assert.GreaterOrEqual(t, time.Since(start), grace)
		assert.Len(t, runs.calls, 1)
	})

	t.Run("gives up on a hung teardown", func(t *testing.T) {
		// Arrange
		iostreamstest.StubOut(t)
		runs := &fakeRuns{release: make(chan struct{})}
		t.Cleanup(func() { close(runs.release) })
		svc := newTestService(runs)
		svc.teardownTimeout = 50 * time.Millisecond
		gone := exec.Command(os.Args[0], "-test.run=^$")
		require.NoError(t, gone.Run())

		// Act
		err := svc.Watch(Target{
			RunRef: sandbox.RunRef{
				ID:       testRunID,
				Runtime:  sandbox.RuntimeDocker,
				Tool:     "claude",
				ToolHome: t.TempDir(),
			},
			PID: gone.Process.Pid,
		})

		// Assert
		require.EqualError(t, err, "teardown of run 0123456789ab not done within 50ms")
	})
}
