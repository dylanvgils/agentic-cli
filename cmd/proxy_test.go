package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/housekeeping"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_resolveProxyEnabled(t *testing.T) {
	enabled := true
	disabled := false

	t.Run("no flag and no config defaults off", func(t *testing.T) {
		// Act
		result := resolveProxyEnabled(runToolCmd, &config.AgenticRC{})

		// Assert
		assert.False(t, result)
	})

	t.Run("config enabled is honored", func(t *testing.T) {
		// Arrange
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &enabled}}}

		// Act
		result := resolveProxyEnabled(runToolCmd, rc)

		// Assert
		assert.True(t, result)
	})

	t.Run("proxy flag overrides config disabled", func(t *testing.T) {
		// Arrange
		require.NoError(t, runToolCmd.Flags().Set("proxy", "true"))
		t.Cleanup(func() {
			_ = runToolCmd.Flags().Set("proxy", "false")
			runToolCmd.Flags().Lookup("proxy").Changed = false
		})
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &disabled}}}

		// Act
		result := resolveProxyEnabled(runToolCmd, rc)

		// Assert
		assert.True(t, result)
	})

	t.Run("no-proxy flag overrides config enabled", func(t *testing.T) {
		// Arrange
		require.NoError(t, runToolCmd.Flags().Set("no-proxy", "true"))
		t.Cleanup(func() {
			_ = runToolCmd.Flags().Set("no-proxy", "false")
			runToolCmd.Flags().Lookup("no-proxy").Changed = false
		})
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &enabled}}}

		// Act
		result := resolveProxyEnabled(runToolCmd, rc)

		// Assert
		assert.False(t, result)
	})

	t.Run("proxy flag explicitly set false does not override config disabled", func(t *testing.T) {
		// Arrange
		require.NoError(t, runToolCmd.Flags().Set("proxy", "false"))
		t.Cleanup(func() {
			runToolCmd.Flags().Lookup("proxy").Changed = false
		})
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &disabled}}}

		// Act
		result := resolveProxyEnabled(runToolCmd, rc)

		// Assert
		assert.False(t, result)
	})
}

func Test_ensureProxyImage(t *testing.T) {
	t.Run("builds the image when missing", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)
		var built string
		stubBuildProxyImage(t, func(image, _, _ string, _ tools.BuildOptions) error {
			built = image
			return nil
		})

		// Act
		err := ensureProxyImage(runToolCmd)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, tools.ProxyImage, built)
	})

	t.Run("skips build when image already exists", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{Image: tools.ProxyImage}, nil)
		built := false
		stubBuildProxyImage(t, func(string, string, string, tools.BuildOptions) error {
			built = true
			return nil
		})

		// Act
		err := ensureProxyImage(runToolCmd)

		// Assert
		require.NoError(t, err)
		assert.False(t, built)
	})
}

func Test_runProxyBuildOrUpdate(t *testing.T) {
	t.Run("build does not force no-cache by default", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubBuildProxyImage(t, func(_, _, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })

		// Act
		err := runProxyBuild(proxyBuildCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.False(t, capturedOpts.NoCache)
	})

	t.Run("build --no-cache forces a fresh build", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubBuildProxyImage(t, func(_, _, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })
		require.NoError(t, proxyBuildCmd.Flags().Set("no-cache", "true"))
		defer proxyBuildCmd.Flags().Set("no-cache", "false") //nolint:errcheck

		// Act
		err := runProxyBuild(proxyBuildCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.True(t, capturedOpts.NoCache)
	})

	t.Run("update always forces no-cache", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubBuildProxyImage(t, func(_, _, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })

		// Act
		err := runProxyUpdate(proxyUpdateCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.True(t, capturedOpts.NoCache)
	})

	t.Run("build never checks for an existing image", func(t *testing.T) {
		// Arrange - no inspectImage stub is set up; if runProxyBuild checked
		// existence first this would panic on the unstubbed real docker call
		built := false
		stubBuildProxyImage(t, func(_, _, _ string, _ tools.BuildOptions) error {
			built = true
			return nil
		})
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })

		// Act
		err := runProxyBuild(proxyBuildCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.True(t, built)
	})

	t.Run("dry run prints dockerfile and skips build", func(t *testing.T) {
		// Arrange
		var built bool
		stubBuildProxyImage(t, func(_, _, _ string, _ tools.BuildOptions) error {
			built = true
			return nil
		})
		require.NoError(t, proxyBuildCmd.Flags().Set("dry-run", "true"))
		defer proxyBuildCmd.Flags().Set("dry-run", "false") //nolint:errcheck

		// Act
		out := captureStdout(t, func() {
			err := runProxyBuild(proxyBuildCmd, nil)
			require.NoError(t, err)
		})

		// Assert
		assert.False(t, built)
		assert.Contains(t, out, "FROM")
	})
}

func Test_proxyRetentionDays(t *testing.T) {
	t.Run("uses the value configured in agentic.json", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		cfg, err := config.LoadConfig(toolHome)
		require.NoError(t, err)
		cfg.ProxyLogRetentionDays = 30
		require.NoError(t, cfg.Save(toolHome))

		// Act
		result := proxyRetentionDays()

		// Assert
		assert.Equal(t, 30, result)
	})

	t.Run("falls back to default when not configured", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)

		// Act
		result := proxyRetentionDays()

		// Assert
		assert.Equal(t, housekeeping.DefaultProxyLogRetentionDays, result)
	})
}

func Test_proxyLogDir(t *testing.T) {
	t.Run("returns empty string when proxy is disabled", func(t *testing.T) {
		// Act
		dir, err := proxyLogDir(false)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, dir)
	})

	t.Run("creates the log dir and prunes old logs", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		cfg, err := config.LoadConfig(toolHome)
		require.NoError(t, err)
		cfg.ProxyLogRetentionDays = 1
		require.NoError(t, cfg.Save(toolHome))
		logDir := filepath.Join(toolHome, "proxy")
		require.NoError(t, os.MkdirAll(logDir, 0o750))
		oldLog := filepath.Join(logDir, "old.jsonl")
		require.NoError(t, os.WriteFile(oldLog, []byte("{}\n"), 0o644))
		old := time.Now().Add(-48 * time.Hour)
		require.NoError(t, os.Chtimes(oldLog, old, old))

		// Act
		dir, err := proxyLogDir(true)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, logDir, dir)
		assert.NoFileExists(t, oldLog)
	})
}

func Test_runProxyClean(t *testing.T) {
	t.Run("removes the proxy image", func(t *testing.T) {
		// Arrange
		var cleaned string
		stubCleanImage(t, func(image string) error {
			cleaned = image
			return nil
		})

		// Act
		err := runProxyClean(proxyCleanCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, tools.ProxyImage, cleaned)
	})

	t.Run("leaves logs alone without --logs", func(t *testing.T) {
		// Arrange
		stubCleanImage(t, func(string) error { return nil })
		pruned := false
		stubPruneProxyLogs(t, func(string, time.Duration) { pruned = true })

		// Act
		err := runProxyClean(proxyCleanCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.False(t, pruned)
	})

	t.Run("--logs wipes all proxy logs regardless of age", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		stubCleanImage(t, func(string) error { return nil })
		var dir string
		var maxAge time.Duration
		stubPruneProxyLogs(t, func(d string, m time.Duration) { dir, maxAge = d, m })
		require.NoError(t, proxyCleanCmd.Flags().Set("logs", "true"))
		t.Cleanup(func() {
			_ = proxyCleanCmd.Flags().Set("logs", "false")
		})

		// Act
		err := runProxyClean(proxyCleanCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(toolHome, "proxy"), dir)
		assert.Zero(t, maxAge)
	})

	t.Run("propagates cleanImage error before touching logs", func(t *testing.T) {
		// Arrange
		stubCleanImage(t, func(string) error { return fmt.Errorf("clean failed") })
		pruned := false
		stubPruneProxyLogs(t, func(string, time.Duration) { pruned = true })
		require.NoError(t, proxyCleanCmd.Flags().Set("logs", "true"))
		t.Cleanup(func() {
			_ = proxyCleanCmd.Flags().Set("logs", "false")
		})

		// Act
		err := runProxyClean(proxyCleanCmd, nil)

		// Assert
		require.Error(t, err)
		assert.False(t, pruned)
	})
}
