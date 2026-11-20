package watch

import (
	"errors"
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
		run := testRun(t, sandbox.RuntimeDocker)
		runs := &fakeRuns{}
		child := startChild(t)
		start := time.Now()

		// Act
		err := newTestService(runs).Watch(Target{
			RunRef: run,
			PID:    child.Process.Pid,
		})

		// Assert
		require.NoError(t, err)
		assert.GreaterOrEqual(t, time.Since(start), childLifetime/2, "waits for the exit")
		assert.Equal(t, ReadyLine+"\n", out.String())
		assert.Equal(t, []sandbox.RunRef{run}, runs.calls)
	})

	t.Run("a process already gone is reaped", func(t *testing.T) {
		// Arrange
		out := iostreamstest.StubOut(t)
		runs := &fakeRuns{}
		pid := goneProcess(t)

		// Act
		err := newTestService(runs).Watch(Target{
			RunRef: testRun(t, sandbox.RuntimeMicrosandbox),
			PID:    pid,
		})

		// Assert
		require.NoError(t, err)
		assert.Empty(t, out.String(), "nobody waits for it to be ready")
		assert.Len(t, runs.calls, 1)
	})

	t.Run("returns the teardown's error", func(t *testing.T) {
		// Arrange
		iostreamstest.StubOut(t)
		runs := &fakeRuns{err: errors.New("stub: teardown failed")}
		pid := goneProcess(t)

		// Act
		err := newTestService(runs).Watch(Target{
			RunRef: testRun(t, sandbox.RuntimeDocker),
			PID:    pid,
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
		pid := goneProcess(t)
		start := time.Now()

		// Act
		err := svc.Watch(Target{
			RunRef: testRun(t, sandbox.RuntimeDocker),
			PID:    pid,
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
		pid := goneProcess(t)

		// Act
		err := svc.Watch(Target{
			RunRef: testRun(t, sandbox.RuntimeDocker),
			PID:    pid,
		})

		// Assert
		require.EqualError(t, err, "teardown of run 0123456789ab not done within 50ms")
	})
}
