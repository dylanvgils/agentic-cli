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
	// Keep the repo's own .agenticrc.toml out of the test
	t.Chdir(t.TempDir())

	t.Run("no cache flag sets opt", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubDocker(t, &fakeDocker{buildTool: func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		}})
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })

		stubFlag(t, buildCmd, "no-cache", "true")

		// Act
		err := runBuild(buildCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.True(t, capturedOpts.NoCache)
	})

	t.Run("skip install checksum flag sets opt", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubDocker(t, &fakeDocker{buildTool: func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		}})
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })

		stubFlag(t, buildCmd, "skip-install-checksum", "true")

		// Act
		err := runBuild(buildCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.True(t, capturedOpts.SkipInstallChecksum)
	})

	t.Run("pull flag sets opt", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubDocker(t, &fakeDocker{buildTool: func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		}})
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })

		stubFlag(t, buildCmd, "pull", "true")

		// Act
		err := runBuild(buildCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.True(t, capturedOpts.Pull)
	})

	t.Run("dry run flag prints dockerfile and skips build", func(t *testing.T) {
		// Arrange
		var buildCalled bool
		stubDocker(t, &fakeDocker{buildTool: func(_, _ string, _ tools.BuildOptions) error {
			buildCalled = true
			return nil
		}})

		stubFlag(t, buildCmd, "dry-run", "true")

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
		stubDocker(t, &fakeDocker{buildTool: func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		}})
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })

		stubFlag(t, buildCmd, "node", "22")

		// Act
		err := runBuild(buildCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "22", capturedOpts.Versions["node"])
	})

}
