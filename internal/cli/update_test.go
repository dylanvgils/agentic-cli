package cli

import (
	"fmt"
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunUpdate(t *testing.T) {
	t.Run("no cache flag sets opt", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubUpdateUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubUpdateInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })

		require.NoError(t, updateCmd.Flags().Set("no-cache", "true"))
		defer updateCmd.Flags().Set("no-cache", "false") //nolint:errcheck

		// Act
		err := runUpdate(updateCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.True(t, capturedOpts.NoCache)
	})

	t.Run("pull flag defaults true", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubUpdateUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubUpdateInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })

		// Act
		err := runUpdate(updateCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.True(t, capturedOpts.Pull)
	})

	t.Run("pull flag can be disabled", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubUpdateUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubUpdateInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })

		require.NoError(t, updateCmd.Flags().Set("pull", "false"))
		defer updateCmd.Flags().Set("pull", "true") //nolint:errcheck

		// Act
		err := runUpdate(updateCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.False(t, capturedOpts.Pull)
	})

	t.Run("stops on first update error", func(t *testing.T) {
		// Arrange
		var updated []string
		stubUpdateUpdateTool(t, func(tool, _ string, _ tools.BuildOptions) error {
			updated = append(updated, tool)
			return fmt.Errorf("fail on %s", tool)
		})
		stubUpdateInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)

		// Act
		err := runUpdate(updateCmd, []string{})

		// Assert
		require.Error(t, err)
		assert.Len(t, updated, 1)
	})

	t.Run("no tools built prints message", func(t *testing.T) {
		// Arrange
		stubUpdateInspectImage(t, nil, nil)

		// Act
		out := captureStdout(t, func() {
			err := runUpdate(updateCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "No tools are built.")
	})

	t.Run("all flag with no images prints message", func(t *testing.T) {
		// Arrange
		stubUpdateListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) { return nil, nil })

		cmd := updateCmd
		require.NoError(t, cmd.Flags().Set("all", "true"))
		defer cmd.Flags().Set("all", "false") //nolint:errcheck

		// Act
		out := captureStdout(t, func() {
			err := runUpdate(cmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "No agentic images found")
	})

	t.Run("all flag updates all images and prunes", func(t *testing.T) {
		// Arrange
		var updated []string
		stubUpdateUpdateTool(t, func(tool, _ string, _ tools.BuildOptions) error {
			updated = append(updated, tool)
			return nil
		})
		stubUpdateInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })
		stubUpdateListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude", Base: "node@24"},
				{Image: "work-copilot", Namespace: "work", Tool: "copilot", Base: "node@24"},
			}, nil
		})

		cmd := updateCmd
		require.NoError(t, cmd.Flags().Set("all", "true"))
		defer cmd.Flags().Set("all", "false") //nolint:errcheck

		// Act
		err := runUpdate(cmd, []string{})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"claude", "copilot"}, updated)
	})

	t.Run("all flag clears rc config base for per-image recovery", func(t *testing.T) {
		// Arrange - simulate an RC config with build.bases = ["java"] in a temp dir.
		// Without the fix, opts.BaseOverride = "java" (from RC) would prevent per-image
		// recovery, and every image would be rebuilt with "java" regardless of its label.
		t.Chdir(t.TempDir())
		require.NoError(t, os.WriteFile(".agenticrc.toml", []byte("[build]\nbases = [\"java\"]\n"), 0o600))

		var capturedOpts []tools.BuildOptions
		stubUpdateUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = append(capturedOpts, opts)
			return nil
		})
		stubUpdateInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })
		stubUpdateListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude", Base: "go@1.23"},
				{Image: "work-copilot", Namespace: "work", Tool: "copilot", Base: "dotnet@8"},
			}, nil
		})

		cmd := updateCmd
		require.NoError(t, cmd.Flags().Set("all", "true"))
		defer cmd.Flags().Set("all", "false") //nolint:errcheck

		// Act
		err := runUpdate(cmd, []string{})

		// Assert - each image uses its own label-recovered base, not "java" from RC
		require.NoError(t, err)
		require.Len(t, capturedOpts, 2)
		assert.NotEqual(t, []string{"java"}, capturedOpts[0].BaseOverride)
		assert.NotEqual(t, []string{"java"}, capturedOpts[1].BaseOverride)
		assert.NotEqual(t, capturedOpts[0].BaseOverride, capturedOpts[1].BaseOverride)
	})

	t.Run("rc config base does not override per-image label for single tool", func(t *testing.T) {
		// Arrange - RC config with build.bases = ["java"]; image was built with go only.
		t.Chdir(t.TempDir())
		require.NoError(t, os.WriteFile(".agenticrc.toml", []byte("[build]\nbases = [\"java\"]\n"), 0o600))

		var capturedOpts tools.BuildOptions
		stubUpdateUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubUpdateInspectImage(t, &docker.ImageInfo{Version: "1.0.0", Base: "go@1.23"}, nil)
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })

		// Act
		err := runUpdate(updateCmd, []string{"claude"})

		// Assert - image's own base recovered from label, not java from RC
		require.NoError(t, err)
		assert.NotEmpty(t, capturedOpts.BaseOverride)
		assert.NotEqual(t, []string{"java"}, capturedOpts.BaseOverride)
	})

	t.Run("rc config apt does not override per-image label for single tool", func(t *testing.T) {
		// Arrange - RC config with build.apt_packages = ["make"]; image was built with cmake only.
		t.Chdir(t.TempDir())
		require.NoError(t, os.WriteFile(".agenticrc.toml", []byte("[build]\napt_packages = [\"make\"]\n"), 0o600))

		var capturedOpts tools.BuildOptions
		stubUpdateUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubUpdateInspectImage(t, &docker.ImageInfo{Version: "1.0.0", Apt: "cmake"}, nil)
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })

		// Act
		err := runUpdate(updateCmd, []string{"claude"})

		// Assert - image's own apt packages recovered from label, RC config apt not injected
		require.NoError(t, err)
		assert.Equal(t, []string{"cmake"}, capturedOpts.AptPackages)
	})

	t.Run("all flag with explicit base flag applies base to all images", func(t *testing.T) {
		// Arrange - use a temp dir (no RC config) so the only base is the explicit flag.
		t.Chdir(t.TempDir())

		var capturedOpts []tools.BuildOptions
		stubUpdateUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = append(capturedOpts, opts)
			return nil
		})
		stubUpdateInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })
		stubUpdateListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude", Base: "node@24"},
				{Image: "work-copilot", Namespace: "work", Tool: "copilot", Base: "node@24,dotnet@8"},
			}, nil
		})

		cmd := updateCmd
		require.NoError(t, cmd.Flags().Set("all", "true"))
		require.NoError(t, cmd.Flags().Set("base", "java"))
		defer func() {
			cmd.Flags().Set("all", "false") //nolint:errcheck
			cmd.Flags().Set("base", "")     //nolint:errcheck
		}()

		// Act
		err := runUpdate(cmd, []string{})

		// Assert - explicit --base java must reach every target unchanged
		require.NoError(t, err)
		require.Len(t, capturedOpts, 2)
		assert.Equal(t, []string{"java"}, capturedOpts[0].BaseOverride)
		assert.Equal(t, []string{"java"}, capturedOpts[1].BaseOverride)
	})

	t.Run("all flag with base env var applies base to all images", func(t *testing.T) {
		// Arrange - AGENTIC_BASE_OVERRIDE is an explicit env override; it must NOT be cleared.
		t.Chdir(t.TempDir())
		t.Setenv(config.EnvBaseOverride, "java")

		var capturedOpts []tools.BuildOptions
		stubUpdateUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = append(capturedOpts, opts)
			return nil
		})
		stubUpdateInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })
		stubUpdateListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude", Base: "node@24"},
				{Image: "work-copilot", Namespace: "work", Tool: "copilot", Base: "node@24,dotnet@8"},
			}, nil
		})

		cmd := updateCmd
		require.NoError(t, cmd.Flags().Set("all", "true"))
		defer cmd.Flags().Set("all", "false") //nolint:errcheck

		// Act
		err := runUpdate(cmd, []string{})

		// Assert - env var override must reach every target unchanged
		require.NoError(t, err)
		require.Len(t, capturedOpts, 2)
		assert.Equal(t, []string{"java"}, capturedOpts[0].BaseOverride)
		assert.Equal(t, []string{"java"}, capturedOpts[1].BaseOverride)
	})

	t.Run("all flag with tool arg updates only that tool across namespaces", func(t *testing.T) {
		// Arrange
		var updated []string
		stubUpdateUpdateTool(t, func(tool, _ string, _ tools.BuildOptions) error {
			updated = append(updated, tool)
			return nil
		})
		stubUpdateInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })
		stubUpdateListAllImages(t, func(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			// Docker would apply the ToolFilter server-side; simulate by honouring it here.
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude", Base: "node@24"},
				{Image: "work-claude", Namespace: "work", Tool: "claude", Base: "node@24"},
			}, nil
		})

		cmd := updateCmd
		require.NoError(t, cmd.Flags().Set("all", "true"))
		defer cmd.Flags().Set("all", "false") //nolint:errcheck

		// Act
		err := runUpdate(cmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"claude", "claude"}, updated)
	})

	t.Run("all flag shares cache-bust value across targets", func(t *testing.T) {
		// Arrange
		var capturedOpts []tools.BuildOptions
		stubUpdateUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = append(capturedOpts, opts)
			return nil
		})
		stubUpdateInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		stubPruneImages(t, func() error { return nil })
		stubPruneBuildCache(t, func() error { return nil })
		stubUpdateListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude", Base: "node@24"},
				{Image: "work-claude", Namespace: "work", Tool: "claude", Base: "node@24"},
			}, nil
		})

		cmd := updateCmd
		require.NoError(t, cmd.Flags().Set("all", "true"))
		defer cmd.Flags().Set("all", "false") //nolint:errcheck

		// Act
		err := runUpdate(cmd, []string{})

		// Assert - same tool rebuilt across two namespaces should reuse the same
		// CacheBust value, so Docker can serve cached tool-stage layers for the second build
		require.NoError(t, err)
		require.Len(t, capturedOpts, 2)
		assert.NotEmpty(t, capturedOpts[0].CacheBust)
		assert.Equal(t, capturedOpts[0].CacheBust, capturedOpts[1].CacheBust)
	})
}
