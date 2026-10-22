package cli

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runProxyBuildOrUpdate(t *testing.T) {
	t.Run("build does not force no-cache by default", func(t *testing.T) {
		// Arrange
		logBuf := stubErrLog(t)
		var capturedOpts tools.BuildOptions
		stubBuildProxyImage(t, func(_, _, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})

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
		stubFlag(t, proxyBuildCmd, "no-cache", "true")

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

		// Act
		err := runProxyUpdate(proxyUpdateCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.True(t, capturedOpts.NoCache)
	})

	t.Run("dry run prints dockerfile and skips build", func(t *testing.T) {
		// Arrange
		var built bool
		stubBuildProxyImage(t, func(_, _, _ string, _ tools.BuildOptions) error {
			built = true
			return nil
		})
		stubFlag(t, proxyBuildCmd, "dry-run", "true")

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
		withTempToolHome(t)
		stubCleanImage(t, func(string) error { return nil })
		log := writeProxyLog(t)

		// Act
		err := runProxyClean(proxyCleanCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.FileExists(t, log)
	})

	t.Run("--logs wipes all proxy logs regardless of age", func(t *testing.T) {
		// Arrange
		logBuf := stubErrLog(t)
		withTempToolHome(t)
		stubCleanImage(t, func(string) error { return nil })
		log := writeProxyLog(t)
		stubFlag(t, proxyCleanCmd, "logs", "true")

		// Act
		err := runProxyClean(proxyCleanCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.NoFileExists(t, log)
		assert.Contains(t, logBuf.String(), "agentic: removing proxy logs")
	})

	t.Run("propagates cleanImage error before touching logs", func(t *testing.T) {
		// Arrange
		withTempToolHome(t)
		stubCleanImage(t, func(string) error { return fmt.Errorf("clean failed") })
		log := writeProxyLog(t)
		stubFlag(t, proxyCleanCmd, "logs", "true")

		// Act
		err := runProxyClean(proxyCleanCmd, nil)

		// Assert
		require.Error(t, err)
		assert.FileExists(t, log)
	})
}
