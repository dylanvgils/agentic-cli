package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunBuild(t *testing.T) {
	t.Run("no cache flag sets opt", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubBuildTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })

		require.NoError(t, buildCmd.Flags().Set("no-cache", "true"))
		defer buildCmd.Flags().Set("no-cache", "false") //nolint:errcheck

		// Act
		err := runBuild(buildCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.True(t, capturedOpts.NoCache)
	})

	t.Run("pull flag defaults false", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubBuildTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })

		// Act
		err := runBuild(buildCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.False(t, capturedOpts.Pull)
	})

	t.Run("pull flag sets opt", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubBuildTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })

		require.NoError(t, buildCmd.Flags().Set("pull", "true"))
		defer buildCmd.Flags().Set("pull", "false") //nolint:errcheck

		// Act
		err := runBuild(buildCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.True(t, capturedOpts.Pull)
	})

	t.Run("base flag sets opt", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		var capturedOpts tools.BuildOptions
		stubBuildTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })

		require.NoError(t, buildCmd.Flags().Set("base", "java"))
		defer buildCmd.Flags().Set("base", "") //nolint:errcheck

		// Act
		err := runBuild(buildCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"java"}, capturedOpts.BaseOverride)
	})

	t.Run("dry run flag prints dockerfile and skips build", func(t *testing.T) {
		// Arrange
		var buildCalled bool
		stubBuildTool(t, func(_, _ string, _ tools.BuildOptions) error {
			buildCalled = true
			return nil
		})

		require.NoError(t, buildCmd.Flags().Set("dry-run", "true"))
		defer buildCmd.Flags().Set("dry-run", "false") //nolint:errcheck

		// Act
		out := captureStdout(t, func() {
			err := runBuild(buildCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.False(t, buildCalled)
		assert.Contains(t, out, "FROM")
	})

	t.Run("invalid project config fails fast with a clear error", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		rcPath := filepath.Join(dir, ".agenticrc.toml")
		require.NoError(t, os.WriteFile(rcPath, []byte("not valid toml [[["), 0o644))
		t.Chdir(dir)

		// Act
		err := runBuild(buildCmd, []string{"claude"})

		// Assert
		assert.ErrorContains(t, err, rcPath)
	})

	t.Run("node flag sets opt", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubBuildTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })

		require.NoError(t, buildCmd.Flags().Set("node", "22"))
		defer buildCmd.Flags().Set("node", "") //nolint:errcheck

		// Act
		err := runBuild(buildCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "22", capturedOpts.Versions["node"])
	})

	t.Run("go flag sets opt", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubBuildTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })

		require.NoError(t, buildCmd.Flags().Set("go", "1.23"))
		defer buildCmd.Flags().Set("go", "") //nolint:errcheck

		// Act
		err := runBuild(buildCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "1.23", capturedOpts.Versions["go"])
	})

}
