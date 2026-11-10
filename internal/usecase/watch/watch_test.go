package watch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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
	t.Run("tears the run down once the process exits and removes its log", func(t *testing.T) {
		// Arrange
		out := iostreamstest.StubOut(t)
		toolHome := t.TempDir()
		writeLog(t, toolHome)
		runs := &fakeRuns{}
		child := startChild(t)
		start := time.Now()

		// Act
		err := New(runs).Watch(Target{
			PID:      child.Process.Pid,
			Runtime:  sandbox.RuntimeDocker,
			ToolHome: toolHome,
			RunID:    testRunID,
		})

		// Assert
		require.NoError(t, err)
		assert.GreaterOrEqual(t, time.Since(start), childLifetime/2, "waits for the exit")
		assert.Equal(t, ReadyLine+"\n", out.String())
		assert.Equal(t, []teardownCall{{runtime: sandbox.RuntimeDocker, toolHome: toolHome, id: testRunID}}, runs.calls)
		assert.NoFileExists(t, LogFile(toolHome, testRunID))
	})

	t.Run("a process already gone is torn down at once", func(t *testing.T) {
		// Arrange
		out := iostreamstest.StubOut(t)
		runs := &fakeRuns{}
		gone := exec.Command(os.Args[0], "-test.run=^$")
		require.NoError(t, gone.Run())

		// Act
		err := New(runs).Watch(Target{
			PID:      gone.Process.Pid,
			Runtime:  sandbox.RuntimeMicrosandbox,
			ToolHome: t.TempDir(),
			RunID:    testRunID,
		})

		// Assert
		require.NoError(t, err)
		assert.Empty(t, out.String(), "nobody waits for it to be ready")
		assert.Len(t, runs.calls, 1)
	})

	t.Run("a failed teardown keeps the log", func(t *testing.T) {
		// Arrange
		iostreamstest.StubOut(t)
		toolHome := t.TempDir()
		writeLog(t, toolHome)
		runs := &fakeRuns{err: errors.New("stub: teardown failed")}
		gone := exec.Command(os.Args[0], "-test.run=^$")
		require.NoError(t, gone.Run())

		// Act
		err := New(runs).Watch(Target{
			PID:      gone.Process.Pid,
			Runtime:  sandbox.RuntimeDocker,
			ToolHome: toolHome,
			RunID:    testRunID,
		})

		// Assert
		require.EqualError(t, err, "stub: teardown failed")
		assert.FileExists(t, LogFile(toolHome, testRunID))
	})
}

func TestLogFile(t *testing.T) {
	// Act
	path := LogFile("/example.test/home", testRunID)

	// Assert
	assert.Equal(t, filepath.Join("/example.test/home", "logs", "watch_"+string(testRunID)+".log"), path)
}

// writeLog creates run testRunID's watchdog log under toolHome, as the spawn does.
func writeLog(t *testing.T, toolHome string) {
	t.Helper()
	path := LogFile(toolHome, testRunID)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, nil, 0o600))
}
