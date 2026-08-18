package run

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/housekeeping"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_proxyAllowList(t *testing.T) {
	t.Run("merges tool baseline with rc-configured hosts", func(t *testing.T) {
		// Arrange
		toolConfig := tools.ToolConfig{Runtime: tools.RuntimeConfig{AllowedHosts: []string{"api.example.com"}}}
		rc := &config.AgenticRC{}
		rc.Run.Proxy.AllowedHosts = []string{"extra.example.com"}

		// Act
		result := proxyAllowList(toolConfig, rc)

		// Assert
		assert.Equal(t, []string{"api.example.com", "extra.example.com"}, result)
	})

	t.Run("empty allowlists on both sides return empty result", func(t *testing.T) {
		// Act
		result := proxyAllowList(tools.ToolConfig{}, &config.AgenticRC{})

		// Assert
		assert.Empty(t, result)
	})
}

func Test_proxyRetentionDays(t *testing.T) {
	t.Run("uses the value configured in agentic.json", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		cfg, err := config.LoadConfig(home)
		require.NoError(t, err)
		cfg.ProxyLogRetentionDays = 30
		require.NoError(t, cfg.Save(home))

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
		dir, err := proxyLogDir(t.TempDir(), false)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, dir)
	})

	t.Run("creates the log dir and prunes old logs", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		cfg, err := config.LoadConfig(home)
		require.NoError(t, err)
		cfg.ProxyLogRetentionDays = 1
		require.NoError(t, cfg.Save(home))
		logDir := filepath.Join(home, "proxy")
		require.NoError(t, os.MkdirAll(logDir, 0o750))
		oldLog := filepath.Join(logDir, "old.jsonl")
		require.NoError(t, os.WriteFile(oldLog, []byte("{}\n"), 0o644))
		old := time.Now().Add(-48 * time.Hour)
		require.NoError(t, os.Chtimes(oldLog, old, old))

		// Act
		dir, err := proxyLogDir(home, true)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, logDir, dir)
		assert.NoFileExists(t, oldLog)
	})
}
