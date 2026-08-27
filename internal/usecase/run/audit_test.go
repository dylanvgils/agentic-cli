package run

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/fswatch"
	"github.com/dylanvgils/agentic-cli/internal/housekeeping"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_auditPaths(t *testing.T) {
	t.Run("expands and cleans the host side of each mount", func(t *testing.T) {
		// Arrange
		volumes := []string{"$TOOL_HOME/claude/data:$CONTAINER_HOME/.claude/", "/a/b:/c"}

		// Act
		result := auditPaths(volumes, "/home/user/.agentic", "/home/claude")

		// Assert
		assert.Equal(t, []string{"/home/user/.agentic/claude/data", "/a/b"}, result)
	})

	t.Run("skips docker named volumes", func(t *testing.T) {
		// Arrange
		volumes := []string{"agentic-data-vol:/data", "/a/b:/c"}

		// Act
		result := auditPaths(volumes, "/home/user/.agentic", "/home/claude")

		// Assert
		assert.Equal(t, []string{"/a/b"}, result)
	})
}

func Test_auditRetentionDays(t *testing.T) {
	t.Run("uses the value configured in agentic.json", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		cfg, err := config.LoadConfig(home)
		require.NoError(t, err)
		cfg.AuditLogRetentionDays = 30
		require.NoError(t, cfg.Save(home))

		// Act
		result := auditRetentionDays(home)

		// Assert
		assert.Equal(t, 30, result)
	})

	t.Run("falls back to default when not configured", func(t *testing.T) {
		// Arrange
		home := t.TempDir()

		// Act
		result := auditRetentionDays(home)

		// Assert
		assert.Equal(t, housekeeping.DefaultAuditLogRetentionDays, result)
	})
}

func Test_auditLogDir(t *testing.T) {
	t.Run("returns empty string when auditing is disabled", func(t *testing.T) {
		// Act
		dir, err := auditLogDir(t.TempDir(), false)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, dir)
	})

	t.Run("creates the log dir and prunes old logs", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		cfg, err := config.LoadConfig(home)
		require.NoError(t, err)
		cfg.AuditLogRetentionDays = 1
		require.NoError(t, cfg.Save(home))
		logDir := filepath.Join(home, config.LogsDirName)
		require.NoError(t, os.MkdirAll(logDir, 0o750))
		oldLog := filepath.Join(logDir, fswatch.LogFilePrefix+"old.jsonl")
		require.NoError(t, os.WriteFile(oldLog, []byte("{}\n"), 0o644))
		old := time.Now().Add(-48 * time.Hour)
		require.NoError(t, os.Chtimes(oldLog, old, old))

		// Act
		dir, err := auditLogDir(home, true)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, logDir, dir)
		assert.NoFileExists(t, oldLog)
	})
}
