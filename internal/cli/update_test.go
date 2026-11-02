package cli

import (
	"fmt"
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/usecase/update"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runUpdate(t *testing.T) {
	// Keep the repo's own .agenticrc.toml out of the test
	t.Chdir(t.TempDir())

	stubUpstream(t, "", false, false)

	t.Run("no cache flag sets opt", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubDocker(t, &fakeDocker{
			buildTool: func(_, _ string, opts tools.BuildOptions) error {
				capturedOpts = opts
				return nil
			},
			inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil),
		})

		stubFlag(t, updateCmd, "no-cache", "true")

		// Act
		err := runUpdate(updateCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.True(t, capturedOpts.NoCache)
	})

	t.Run("skip install checksum flag sets opt", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubDocker(t, &fakeDocker{
			buildTool: func(_, _ string, opts tools.BuildOptions) error {
				capturedOpts = opts
				return nil
			},
			inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil),
		})

		stubFlag(t, updateCmd, "skip-install-checksum", "true")

		// Act
		err := runUpdate(updateCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.True(t, capturedOpts.SkipInstallChecksum)
	})

	t.Run("pull flag defaults true", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubDocker(t, &fakeDocker{
			buildTool: func(_, _ string, opts tools.BuildOptions) error {
				capturedOpts = opts
				return nil
			},
			inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil),
		})

		// Act
		err := runUpdate(updateCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.True(t, capturedOpts.Pull)
	})

	t.Run("pull flag can be disabled", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubDocker(t, &fakeDocker{
			buildTool: func(_, _ string, opts tools.BuildOptions) error {
				capturedOpts = opts
				return nil
			},
			inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil),
		})

		stubFlag(t, updateCmd, "pull", "false")

		// Act
		err := runUpdate(updateCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.False(t, capturedOpts.Pull)
	})

	t.Run("dry run flag prints dockerfile and skips update", func(t *testing.T) {
		// Arrange
		var updateCalled bool
		stubDocker(t, &fakeDocker{
			buildTool: func(_, _ string, _ tools.BuildOptions) error {
				updateCalled = true
				return nil
			},
		})

		stubFlag(t, updateCmd, "dry-run", "true")

		// Act
		out := captureLog(t, func() {
			err := runUpdate(updateCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.False(t, updateCalled)
		assert.Contains(t, out, "FROM")
	})

	t.Run("stops on first update error", func(t *testing.T) {
		// Arrange
		var updated []string
		stubDocker(t, &fakeDocker{
			buildTool: func(tool, _ string, _ tools.BuildOptions) error {
				updated = append(updated, tool)
				return fmt.Errorf("fail on %s", tool)
			},
			inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil),
			listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
				return []*docker.ImageInfo{
					{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
					{Image: "agentic-copilot", Namespace: "agentic", Tool: "copilot"},
				}, nil
			},
		})
		stubFlag(t, updateCmd, allFlagName, "true")

		// Act
		err := runUpdate(updateCmd, []string{})

		// Assert
		require.Error(t, err)
		assert.Len(t, updated, 1)
	})

	t.Run("all without images prints message", func(t *testing.T) {
		// Arrange
		stubDocker(t, &fakeDocker{})
		stubFlag(t, updateCmd, allFlagName, "true")
		logBuf := stubErrLog(t)

		// Act
		err := runUpdate(updateCmd, []string{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, logBuf.String(), "agentic: no agentic images found")
	})

	t.Run("all flag updates all images", func(t *testing.T) {
		// Arrange
		logBuf := stubErrLog(t)
		var updated []string
		stubDocker(t, &fakeDocker{
			buildTool: func(tool, _ string, _ tools.BuildOptions) error {
				updated = append(updated, tool)
				return nil
			},
			inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil),
			listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
				return []*docker.ImageInfo{
					{Image: "agentic-claude", Namespace: "agentic", Tool: "claude", Base: "node@24"},
					{Image: "work-copilot", Namespace: "work", Tool: "copilot", Base: "node@24"},
				}, nil
			},
		})

		cmd := updateCmd
		stubFlag(t, cmd, allFlagName, "true")

		// Act
		err := runUpdate(cmd, []string{})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"claude", "copilot"}, updated)
		assert.Contains(t, logBuf.String(), "agentic: updating 2 image(s): ")
	})

	t.Run("base-exact overrides rc bases and per-image label recovery, even when empty", func(t *testing.T) {
		// Arrange - RC config with build.bases = ["java"]; image was built with go only.
		t.Chdir(t.TempDir())
		require.NoError(t, os.WriteFile(".agenticrc.toml", []byte("[build]\nbases = [\"java\"]\n"), 0o600))

		var capturedOpts tools.BuildOptions
		stubDocker(t, &fakeDocker{
			buildTool: func(_, _ string, opts tools.BuildOptions) error {
				capturedOpts = opts
				return nil
			},
			inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0", Base: "go@1.23"}, nil),
		})

		stubFlag(t, updateCmd, "base-exact", "")

		// Act
		err := runUpdate(updateCmd, []string{"claude"})

		// Assert - neither rc's "java" nor the image's recovered "go" survive
		require.NoError(t, err)
		assert.Empty(t, capturedOpts.BaseOverride)
		assert.True(t, capturedOpts.BaseExact)
	})

	t.Run("apt-exact overrides rc apt and per-image label recovery, even when empty", func(t *testing.T) {
		// Arrange - RC config with build.apt_packages = ["make"]; image was built with cmake only.
		t.Chdir(t.TempDir())
		require.NoError(t, os.WriteFile(".agenticrc.toml", []byte("[build]\napt_packages = [\"make\"]\n"), 0o600))

		var capturedOpts tools.BuildOptions
		stubDocker(t, &fakeDocker{
			buildTool: func(_, _ string, opts tools.BuildOptions) error {
				capturedOpts = opts
				return nil
			},
			inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0", Apt: "cmake"}, nil),
		})

		stubFlag(t, updateCmd, "apt-exact", "")

		// Act
		err := runUpdate(updateCmd, []string{"claude"})

		// Assert - neither rc's "make" nor the image's recovered "cmake" survive
		require.NoError(t, err)
		assert.Empty(t, capturedOpts.AptPackages)
		assert.True(t, capturedOpts.AptExact)
	})
}

func Test_logUpdateSummary(t *testing.T) {
	// Arrange
	logBuf := stubLogs(t)
	targets := []update.Target{{Name: "claude", Image: "agentic-claude"}, {Name: "copilot", Image: "agentic-copilot"}}

	// Act
	logUpdateSummary(targets)

	// Assert
	assert.Contains(t, logBuf.String(), "updating 2 image(s): agentic-claude, agentic-copilot")
}

// Test_updateOptsFromFlags only checks which flags count as given; the rule itself is covered by TestUpdateOptions in internal/usecase/resolve.
func Test_updateOptsFromFlags(t *testing.T) {
	withTempToolHome(t)
	rc := &config.AgenticRC{Build: config.RCBuild{Bases: []string{"java"}, AptPackages: []string{"jq"}}}

	t.Run("no flags drops the config bases and apt", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		addBuildFlags(cmd)

		// Act
		opts := updateOptsFromFlags(cmd, rc)

		// Assert
		assert.Nil(t, opts.BaseOverride)
		assert.Nil(t, opts.AptPackages)
	})

	t.Run("each flag pair counts on its own", func(t *testing.T) {
		// Arrange
		cmd := &cobra.Command{Use: "test"}
		addBuildFlags(cmd)
		require.NoError(t, cmd.Flags().Set("apt-exact", "curl"))

		// Act
		opts := updateOptsFromFlags(cmd, rc)

		// Assert - --apt-exact sets apt only; bases still come from the image
		assert.Nil(t, opts.BaseOverride)
		assert.Equal(t, []string{"curl"}, opts.AptPackages)
	})
}
