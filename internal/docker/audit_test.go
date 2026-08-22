package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupAudit(t *testing.T) {
	t.Run("disabled is a no-op", func(t *testing.T) {
		// Act
		cleanup, err := setupAudit(RunSpec{AuditEnabled: false})

		// Assert
		require.NoError(t, err)
		assert.NotPanics(t, cleanup)
	})

	t.Run("dry run is a no-op even when enabled", func(t *testing.T) {
		// Act
		cleanup, err := setupAudit(RunSpec{AuditEnabled: true, DryRun: true})

		// Assert
		require.NoError(t, err)
		assert.NotPanics(t, cleanup)
	})

	t.Run("enabled watches the given paths and logs activity", func(t *testing.T) {
		// Arrange
		root := t.TempDir()
		logDir := t.TempDir()
		rs := RunSpec{AuditEnabled: true, AuditPaths: []string{root}, AuditLogDir: logDir}

		// Act
		cleanup, err := setupAudit(rs)
		require.NoError(t, err)

		target := filepath.Join(root, "file.txt")
		require.NoError(t, os.WriteFile(target, []byte("hi"), 0o644))
		waitForAuditWrite(t, logDir, target)
		cleanup()

		// Assert
		entries, err := os.ReadDir(logDir)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		content, err := os.ReadFile(filepath.Join(logDir, entries[0].Name()))
		require.NoError(t, err)
		assert.Contains(t, string(content), target)
	})
}

func TestAuditHandlePrintSummary(t *testing.T) {
	writeLog := func(t *testing.T, lines ...string) string {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "run.jsonl")
		require.NoError(t, os.WriteFile(logPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644))
		return logPath
	}

	t.Run("reports counts by op when the log has activity", func(t *testing.T) {
		// Arrange
		logPath := writeLog(t,
			`{"op":"open","path":"/workspace/a"}`,
			`{"op":"write","path":"/workspace/a"}`,
			`{"op":"write","path":"/workspace/b"}`,
		)
		handle := auditHandle{logPath: logPath}
		var buf strings.Builder

		// Act
		handle.PrintSummary(&buf)

		// Assert
		assert.Contains(t, buf.String(), "1 open(s)")
		assert.Contains(t, buf.String(), "2 write(s)")
		assert.Contains(t, buf.String(), logPath)
	})

	t.Run("reports a warning for detail-only meta entries even with zero ops", func(t *testing.T) {
		// Arrange
		logPath := writeLog(t, `{"detail":"root could not be watched: permission denied"}`)
		handle := auditHandle{logPath: logPath}
		var buf strings.Builder

		// Act
		handle.PrintSummary(&buf)

		// Assert
		assert.Contains(t, buf.String(), "1 warning(s)")
		assert.Contains(t, buf.String(), logPath)
	})

	t.Run("reports both counts and warnings when both are present", func(t *testing.T) {
		// Arrange
		logPath := writeLog(t,
			`{"op":"write","path":"/workspace/a"}`,
			`{"detail":"inotify queue overflow: some events were dropped"}`,
		)
		handle := auditHandle{logPath: logPath}
		var buf strings.Builder

		// Act
		handle.PrintSummary(&buf)

		// Assert
		assert.Contains(t, buf.String(), "1 write(s)")
		assert.Contains(t, buf.String(), "1 warning(s)")
	})

	t.Run("missing log prints nothing", func(t *testing.T) {
		// Arrange
		handle := auditHandle{logPath: filepath.Join(t.TempDir(), "absent.jsonl")}
		var buf strings.Builder

		// Act
		handle.PrintSummary(&buf)

		// Assert
		assert.Empty(t, buf.String())
	})
}

// waitForAuditWrite polls logDir until it contains a *.jsonl file mentioning
// target, or fails the test after timeout.
func waitForAuditWrite(t *testing.T, logDir, target string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(logDir)
		require.NoError(t, err)
		for _, e := range entries {
			content, err := os.ReadFile(filepath.Join(logDir, e.Name()))
			require.NoError(t, err)
			if strings.Contains(string(content), target) {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s to appear in audit log", target)
}
