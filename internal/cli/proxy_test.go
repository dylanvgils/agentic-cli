package cli

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runProxyBuildOrUpdate(t *testing.T) {
	for _, tc := range []struct {
		name        string
		run         func(*cobra.Command, []string) error
		cmd         *cobra.Command
		noCacheFlag bool
		wantNoCache bool
	}{
		{name: "build does not force no-cache by default", run: runProxyBuild, cmd: proxyBuildCmd},
		{name: "build --no-cache forces a fresh build", run: runProxyBuild, cmd: proxyBuildCmd, noCacheFlag: true, wantNoCache: true},
		{name: "update always forces no-cache", run: runProxyUpdate, cmd: proxyUpdateCmd, wantNoCache: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			logBuf := iostreamstest.StubErr(t)
			var capturedOpts tools.BuildOptions
			stubBuildProxyImage(t, func(_, _, _ string, opts tools.BuildOptions) error {
				capturedOpts = opts
				return nil
			})
			if tc.noCacheFlag {
				stubFlag(t, tc.cmd, "no-cache", "true")
			}

			// Act
			err := tc.run(tc.cmd, nil)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.wantNoCache, capturedOpts.NoCache)
			assert.Contains(t, logBuf.String(), "agentic-proxy")
		})
	}

	t.Run("dry run prints dockerfile and skips build", func(t *testing.T) {
		// Arrange
		var built bool
		stubBuildProxyImage(t, func(_, _, _ string, _ tools.BuildOptions) error {
			built = true
			return nil
		})
		stubFlag(t, proxyBuildCmd, "dry-run", "true")

		// Act
		out := iostreamstest.CaptureOut(t, func() {
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
		logBuf := iostreamstest.StubErr(t)
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
		logBuf := iostreamstest.StubErr(t)
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
		require.ErrorContains(t, err, "clean failed")
		assert.FileExists(t, log)
	})
}
