package run

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config/agentichome"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Service_openRunLog(t *testing.T) {
	t.Run("creates the log dir, prunes old dirs and opens the command log in it", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		require.NoError(t, os.WriteFile(agentichome.ConfigFile(home), []byte(`{"log_retention_days":1}`), 0o640))
		oldDir := logging.Dir(home, "ba9876543210", time.Now().Add(-48*time.Hour))
		require.NoError(t, os.MkdirAll(oldDir, 0o750))
		logDir := logging.Dir(home, "0123456789ab", time.Now())
		l := &fakeCommandLog{}
		svc := New(&fakeDocker{}, nil, &fakeToolUpdates{}, &fakeSidecarImages{}, &fakeWatchdog{}, l)

		// Act
		err := svc.openRunLog(logDir, home)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, logDir)
		assert.NoDirExists(t, oldDir)
		assert.Equal(t, []string{filepath.Join(logDir, commandLogFileName)}, l.opened)
	})

	t.Run("a command log that won't open only warns", func(t *testing.T) {
		// Arrange
		errOut := iostreamstest.StubErr(t)
		home := t.TempDir()
		l := &fakeCommandLog{openErr: errors.New("stub: open failed")}
		svc := New(&fakeDocker{}, nil, &fakeToolUpdates{}, &fakeSidecarImages{}, &fakeWatchdog{}, l)

		// Act
		err := svc.openRunLog(logging.Dir(home, "0123456789ab", time.Now()), home)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, errOut.String(), "the run's commands won't be logged: stub: open failed")
	})

	t.Run("a log dir that can't be created is an error", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		blocker := filepath.Join(home, "file")
		require.NoError(t, os.WriteFile(blocker, nil, 0o600))
		l := &fakeCommandLog{}
		svc := New(&fakeDocker{}, nil, &fakeToolUpdates{}, &fakeSidecarImages{}, &fakeWatchdog{}, l)

		// Act
		err := svc.openRunLog(filepath.Join(blocker, "logs"), home)

		// Assert
		require.ErrorContains(t, err, "create log dir")
		assert.Empty(t, l.opened)
	})
}

func Test_runLogDir(t *testing.T) {
	// Arrange
	run := sandbox.RunRef{
		ID:       "0123456789ab",
		ToolHome: "/example.test/home",
	}
	start := time.Date(2026, 11, 20, 15, 30, 45, 0, time.Local)

	// Act
	dir := runLogDir(run, start)

	// Assert
	assert.Equal(t, filepath.Join("/example.test/home", logging.DirName, "20261120-153045-0123456789ab"), dir)
}
