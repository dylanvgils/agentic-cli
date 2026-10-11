package run

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/housekeeping"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_proxyRetentionDays(t *testing.T) {
	t.Run("uses the value configured in agentic.json", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		require.NoError(t, os.WriteFile(config.ConfigFile(home), []byte(`{"proxy_log_retention_days":30}`), 0o640))

		// Act
		result := proxyRetentionDays(home)

		// Assert
		assert.Equal(t, 30, result)
	})

	t.Run("falls back to default when not configured", func(t *testing.T) {
		// Arrange
		home := t.TempDir()

		// Act
		result := proxyRetentionDays(home)

		// Assert
		assert.Equal(t, housekeeping.DefaultProxyLogRetentionDays, result)
	})
}

func Test_proxyLogDir(t *testing.T) {
	t.Run("returns empty string when proxy is disabled", func(t *testing.T) {
		// Act
		dir := proxyLogDir("/example.test/home", false)

		// Assert
		assert.Empty(t, dir)
	})

	t.Run("returns the logs dir in the tool home without creating it", func(t *testing.T) {
		// Arrange
		home := t.TempDir()

		// Act
		dir := proxyLogDir(home, true)

		// Assert
		assert.Equal(t, filepath.Join(home, config.LogsDirName), dir)
		assert.NoDirExists(t, dir)
	})
}

func Test_prepareProxyLogDir(t *testing.T) {
	t.Run("creates the log dir", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		logDir := filepath.Join(home, config.LogsDirName)

		// Act
		err := prepareProxyLogDir(logDir, home)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, logDir)
	})

	t.Run("prunes logs older than the retention window", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		require.NoError(t, os.WriteFile(config.ConfigFile(home), []byte(`{"proxy_log_retention_days":1}`), 0o640))
		logDir := filepath.Join(home, config.LogsDirName)
		require.NoError(t, os.MkdirAll(logDir, 0o750))
		oldLog := filepath.Join(logDir, proxy.LogFilePrefix+"old.jsonl")
		require.NoError(t, os.WriteFile(oldLog, []byte("{}\n"), 0o644))
		old := time.Now().Add(-48 * time.Hour)
		require.NoError(t, os.Chtimes(oldLog, old, old))

		// Act
		err := prepareProxyLogDir(logDir, home)

		// Assert
		require.NoError(t, err)
		assert.NoFileExists(t, oldLog)
	})
}
