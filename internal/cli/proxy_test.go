package cli

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_resolveProxyMode only needs to confirm each flag is read off cmd and
// wired into the right resolve.ProxyInput field - the actual precedence
// matrix is covered by TestProxyMode in internal/usecase/resolve.
func Test_resolveProxyMode(t *testing.T) {
	t.Run("no flags reads rc value", func(t *testing.T) {
		// Arrange
		enabled := true
		rc := &config.AgenticRC{Run: config.RCRun{Proxy: config.RCProxy{Enabled: &enabled}}}

		// Act
		gotEnabled, gotMonitor := resolveProxyMode(runToolCmd, rc)

		// Assert
		assert.True(t, gotEnabled)
		assert.False(t, gotMonitor)
	})

	t.Run("no-proxy flag propagates", func(t *testing.T) {
		// Arrange
		require.NoError(t, runToolCmd.Flags().Set("no-proxy", "true"))
		t.Cleanup(func() {
			_ = runToolCmd.Flags().Set("no-proxy", "false")
			runToolCmd.Flags().Lookup("no-proxy").Changed = false
		})

		// Act
		gotEnabled, gotMonitor := resolveProxyMode(runToolCmd, &config.AgenticRC{})

		// Assert
		assert.False(t, gotEnabled)
		assert.False(t, gotMonitor)
	})

	t.Run("proxy-monitor flag propagates", func(t *testing.T) {
		// Arrange
		require.NoError(t, runToolCmd.Flags().Set("proxy-monitor", "true"))
		t.Cleanup(func() {
			_ = runToolCmd.Flags().Set("proxy-monitor", "false")
			runToolCmd.Flags().Lookup("proxy-monitor").Changed = false
		})

		// Act
		gotEnabled, gotMonitor := resolveProxyMode(runToolCmd, &config.AgenticRC{})

		// Assert
		assert.True(t, gotEnabled)
		assert.True(t, gotMonitor)
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

	t.Run("skips build when image already exists at current version", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{Image: tools.ProxyImage, CLIVersion: buildinfo.Version}, nil)
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

	t.Run("rebuilds when CLI version does not match image label", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{Image: tools.ProxyImage, CLIVersion: "v0.0.0"}, nil)
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
