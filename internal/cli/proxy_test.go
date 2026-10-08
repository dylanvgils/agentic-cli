package cli

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_proxyInput only confirms each flag maps to the right resolve.ProxyInput field; precedence is covered by TestProxyMode in internal/usecase/resolve.
func Test_proxyInput(t *testing.T) {
	t.Run("no flags leaves every field unset", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		addProxyFlags(cmd)

		// Act
		result := proxyInput(cmd)

		// Assert
		assert.Equal(t, resolve.ProxyInput{}, result)
	})

	t.Run("each flag maps to its field", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		addProxyFlags(cmd)
		require.NoError(t, cmd.Flags().Set("no-proxy", "true"))
		require.NoError(t, cmd.Flags().Set("proxy-monitor", "true"))
		require.NoError(t, cmd.Flags().Set("proxy", "true"))

		// Act
		result := proxyInput(cmd)

		// Assert
		assert.Equal(t, resolve.ProxyInput{NoProxy: true, MonitorFlag: true, ProxyFlag: true}, result)
	})
}

func Test_runProxyBuildOrUpdate(t *testing.T) {
	t.Run("build does not force no-cache by default", func(t *testing.T) {
		// Arrange
		logBuf := stubErrLog(t)
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
		assert.Contains(t, logBuf.String(), "agentic: building agentic-proxy")
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

func Test_runProxyClean(t *testing.T) {
	t.Run("removes the proxy image", func(t *testing.T) {
		// Arrange
		logBuf := stubErrLog(t)
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
		assert.Contains(t, logBuf.String(), "agentic: removing agentic-proxy")
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
		logBuf := stubErrLog(t)
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
		assert.Equal(t, filepath.Join(toolHome, config.LogsDirName), dir)
		assert.Zero(t, maxAge)
		assert.Contains(t, logBuf.String(), "agentic: removing proxy logs")
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
