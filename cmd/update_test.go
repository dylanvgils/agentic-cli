package cmd

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

func TestDryRunUpdate(t *testing.T) {
	t.Run("prints dockerfile skips script", func(t *testing.T) {
		// Arrange
		var scriptCalled bool
		stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error {
			scriptCalled = true
			return nil
		})

		// Act
		out := captureStdout(t, func() {
			err := dryRunUpdate([]string{"claude"}, "agentic", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.False(t, scriptCalled)
		assert.Contains(t, out, "FROM")
	})

	t.Run("without tool arg returns error", func(t *testing.T) {
		// Act
		err := dryRunUpdate([]string{}, "agentic", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--dry-run requires a tool argument")
	})

	t.Run("recovers base from image label", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{Base: "node@24.0.0,java@21.0.1"}, nil)

		// Act
		out := captureStdout(t, func() {
			err := dryRunUpdate([]string{"claude"}, "agentic", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "temurin")
	})

	t.Run("explicit base flag takes precedence", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{Base: "node@24.0.0,go@1.22"}, nil)
		opts := tools.BuildOptions{BaseOverride: []string{"java"}, Versions: map[string]string{}}

		// Act
		out := captureStdout(t, func() {
			err := dryRunUpdate([]string{"claude"}, "agentic", opts)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "temurin")
		assert.NotContains(t, out, "go.dev")
	})

}

func Test_resolveScopedUpdateTargets(t *testing.T) {
	t.Run("single tool always included even if unbuilt", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)

		// Act
		targets, err := resolveScopedUpdateTargets([]string{"claude"}, "agentic", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, "claude", targets[0].name)
	})

	t.Run("mixed built recovers opts from label for built tools", func(t *testing.T) {
		// Arrange
		callCount := 0
		orig := inspectImage
		inspectImage = func(_ string) (*docker.ImageInfo, error) {
			callCount++
			if callCount == 1 {
				return nil, nil // claude not built
			}
			return &docker.ImageInfo{Version: "1.0.0", Base: "node@24,java@21"}, nil
		}
		t.Cleanup(func() { inspectImage = orig })

		// Act
		targets, err := resolveScopedUpdateTargets([]string{}, "agentic", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.NoError(t, err)
		assert.Len(t, targets, 2) // copilot + opencode
		assert.NotEmpty(t, targets[0].opts.BaseOverride)
	})

	t.Run("inspectImage error propagates", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, fmt.Errorf("daemon not running"))

		// Act
		_, err := resolveScopedUpdateTargets([]string{"claude"}, "agentic", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.Error(t, err)
	})
}

func Test_resolveAllUpdateTargets(t *testing.T) {
	t.Run("skips images with empty tool field", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-base", Namespace: "agentic", Tool: ""},
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
			}, nil
		})

		// Act
		targets, err := resolveAllUpdateTargets([]string{}, tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, "claude", targets[0].name)
	})

	t.Run("recovers base independently from each image label", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude", Base: "java@21"},
				{Image: "work-copilot", Namespace: "work", Tool: "copilot", Base: "dotnet@8"},
			}, nil
		})

		// Act
		targets, err := resolveAllUpdateTargets([]string{}, tools.BuildOptions{Versions: map[string]string{}})

		// Assert — each target gets its own label-recovered base, not a shared one
		require.NoError(t, err)
		require.Len(t, targets, 2)
		assert.NotEmpty(t, targets[0].opts.BaseOverride)
		assert.NotEmpty(t, targets[1].opts.BaseOverride)
		assert.NotEqual(t, targets[0].opts.BaseOverride, targets[1].opts.BaseOverride)
	})

	t.Run("recovers apt independently from each image label", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude", Apt: "make,gcc"},
				{Image: "work-copilot", Namespace: "work", Tool: "copilot", Apt: "cmake"},
			}, nil
		})

		// Act
		targets, err := resolveAllUpdateTargets([]string{}, tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 2)
		assert.Equal(t, []string{"make", "gcc"}, targets[0].opts.AptPackages)
		assert.Equal(t, []string{"cmake"}, targets[1].opts.AptPackages)
	})

	t.Run("listAllImages error propagates", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, fmt.Errorf("docker daemon not running")
		})

		// Act
		_, err := resolveAllUpdateTargets([]string{}, tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})

	t.Run("filters to matching tool when args provided", func(t *testing.T) {
		// Arrange
		var capturedFilters []docker.ImageFilter
		stubListAllImages(t, func(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			capturedFilters = filters
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
			}, nil
		})

		// Act
		targets, err := resolveAllUpdateTargets([]string{"claude"}, tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, "claude", targets[0].name)
		assert.Equal(t, []docker.ImageFilter{docker.ToolFilter("claude")}, capturedFilters)
	})
}

func TestUpdateOneTool(t *testing.T) {
	t.Run("version changed reported", func(t *testing.T) {
		// Arrange
		stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })

		callCount := 0
		orig := inspectImage
		inspectImage = func(_ string) (*docker.ImageInfo, error) {
			callCount++
			if callCount == 1 {
				return &docker.ImageInfo{Version: "1.0.0"}, nil // before
			}
			return &docker.ImageInfo{Version: "2.0.0"}, nil // after
		}
		defer func() { inspectImage = orig }()

		// Act
		out := captureStdout(t, func() {
			err := updateOneTool("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "   version: 1.0.0 -> 2.0.0")
	})

	t.Run("version up to date reported", func(t *testing.T) {
		// Arrange
		stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)

		// Act
		out := captureStdout(t, func() {
			err := updateOneTool("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "   version: 1.0.0 (up to date)")
	})

	t.Run("base override shown", func(t *testing.T) {
		// Arrange
		stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		opts := tools.BuildOptions{BaseOverride: []string{"java"}, Versions: map[string]string{}}

		// Act
		out := captureStdout(t, func() {
			err := updateOneTool("claude", "agentic-claude", opts)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "   base: java")
	})

	t.Run("base override hidden when empty", func(t *testing.T) {
		// Arrange
		stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)

		// Act
		out := captureStdout(t, func() {
			err := updateOneTool("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.NotContains(t, out, "=> base:")
	})

	t.Run("apt packages shown", func(t *testing.T) {
		// Arrange
		stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		opts := tools.BuildOptions{AptPackages: []string{"curl", "jq"}, Versions: map[string]string{}}

		// Act
		out := captureStdout(t, func() {
			err := updateOneTool("claude", "agentic-claude", opts)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "   apt: curl, jq")
	})

	t.Run("apt packages hidden when empty", func(t *testing.T) {
		// Arrange
		stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)

		// Act
		out := captureStdout(t, func() {
			err := updateOneTool("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.NotContains(t, out, "apt:")
	})

	t.Run("script error propagates", func(t *testing.T) {
		// Arrange
		stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error {
			return fmt.Errorf("docker daemon not running")
		})
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)

		// Act
		err := updateOneTool("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})
}

func Test_reportVersionChange(t *testing.T) {
	t.Run("version changed", func(t *testing.T) {
		// Act
		out := captureStdout(t, func() { reportVersionChange("1.0.0", "2.0.0") })

		// Assert
		assert.Contains(t, out, "1.0.0 -> 2.0.0")
	})

	t.Run("version up to date", func(t *testing.T) {
		// Act
		out := captureStdout(t, func() { reportVersionChange("1.0.0", "1.0.0") })

		// Assert
		assert.Contains(t, out, "(up to date)")
	})

	t.Run("no before version just prints version", func(t *testing.T) {
		// Act
		out := captureStdout(t, func() { reportVersionChange("", "1.0.0") })

		// Assert
		assert.Contains(t, out, "1.0.0")
		assert.NotContains(t, out, "(up to date)")
	})

	t.Run("no after version prints nothing", func(t *testing.T) {
		// Act
		out := captureStdout(t, func() { reportVersionChange("1.0.0", "") })

		// Assert
		assert.Empty(t, out)
	})
}

func Test_recoverOpts(t *testing.T) {
	t.Run("recovers base from label when not explicitly set", func(t *testing.T) {
		// Act
		result := recoverOpts(&docker.ImageInfo{Base: "node@24,java@21"}, tools.BuildOptions{})

		// Assert
		assert.NotEmpty(t, result.BaseOverride)
	})

	t.Run("explicit base takes precedence", func(t *testing.T) {
		// Act
		result := recoverOpts(&docker.ImageInfo{Base: "node@24,go@1.22"}, tools.BuildOptions{BaseOverride: []string{"java"}})

		// Assert
		assert.Equal(t, []string{"java"}, result.BaseOverride)
	})

	t.Run("recovers apt from label when not explicitly set", func(t *testing.T) {
		// Act
		result := recoverOpts(&docker.ImageInfo{Base: "node@24", Apt: "make,gcc"}, tools.BuildOptions{})

		// Assert
		assert.NotEmpty(t, result.AptPackages)
	})

	t.Run("explicit apt merged with recovered packages", func(t *testing.T) {
		// Act
		result := recoverOpts(&docker.ImageInfo{Base: "node@24", Apt: "make,gcc"}, tools.BuildOptions{AptPackages: []string{"cmake"}})

		// Assert
		assert.Equal(t, []string{"make", "gcc", "cmake"}, result.AptPackages)
	})
}

func TestRunUpdate(t *testing.T) {
	t.Run("no cache flag sets opt", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		stubUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		stubPruneImages(t, func() (string, error) { return "", nil })

		require.NoError(t, updateCmd.Flags().Set("no-cache", "true"))
		defer updateCmd.Flags().Set("no-cache", "false") //nolint:errcheck

		// Act
		err := runUpdate(updateCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.True(t, capturedOpts.NoCache)
	})

	t.Run("prune message shown when reclaimed non zero", func(t *testing.T) {
		// Arrange
		stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		stubPruneImages(t, func() (string, error) { return "512MB", nil })

		// Act
		out := captureStdout(t, func() {
			err := runUpdate(updateCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "=> pruned dangling images (reclaimed 512MB)")
	})

	t.Run("stops on first update error", func(t *testing.T) {
		// Arrange
		var updated []string
		stubUpdateTool(t, func(tool, _ string, _ tools.BuildOptions) error {
			updated = append(updated, tool)
			return fmt.Errorf("fail on %s", tool)
		})
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)

		// Act
		err := runUpdate(updateCmd, []string{})

		// Assert
		require.Error(t, err)
		assert.Len(t, updated, 1)
	})

	t.Run("no tools built prints message", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)

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
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) { return nil, nil })

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
		stubUpdateTool(t, func(tool, _ string, _ tools.BuildOptions) error {
			updated = append(updated, tool)
			return nil
		})
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		stubPruneImages(t, func() (string, error) { return "", nil })
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
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
		// Arrange — simulate an RC config with build.bases = ["java"] in a temp dir.
		// Without the fix, opts.BaseOverride = "java" (from RC) would prevent per-image
		// recovery, and every image would be rebuilt with "java" regardless of its label.
		t.Chdir(t.TempDir())
		require.NoError(t, os.WriteFile(".agenticrc.toml", []byte("[build]\nbases = [\"java\"]\n"), 0o600))

		var capturedOpts []tools.BuildOptions
		stubUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = append(capturedOpts, opts)
			return nil
		})
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		stubPruneImages(t, func() (string, error) { return "", nil })
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
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

		// Assert — each image uses its own label-recovered base, not "java" from RC
		require.NoError(t, err)
		require.Len(t, capturedOpts, 2)
		assert.NotEqual(t, []string{"java"}, capturedOpts[0].BaseOverride)
		assert.NotEqual(t, []string{"java"}, capturedOpts[1].BaseOverride)
		assert.NotEqual(t, capturedOpts[0].BaseOverride, capturedOpts[1].BaseOverride)
	})

	t.Run("rc config base does not override per-image label for single tool", func(t *testing.T) {
		// Arrange — RC config with build.bases = ["java"]; image was built with go only.
		t.Chdir(t.TempDir())
		require.NoError(t, os.WriteFile(".agenticrc.toml", []byte("[build]\nbases = [\"java\"]\n"), 0o600))

		var capturedOpts tools.BuildOptions
		stubUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0", Base: "go@1.23"}, nil)
		stubPruneImages(t, func() (string, error) { return "", nil })

		// Act
		err := runUpdate(updateCmd, []string{"claude"})

		// Assert — image's own base recovered from label, not java from RC
		require.NoError(t, err)
		assert.NotEmpty(t, capturedOpts.BaseOverride)
		assert.NotEqual(t, []string{"java"}, capturedOpts.BaseOverride)
	})

	t.Run("rc config apt does not override per-image label for single tool", func(t *testing.T) {
		// Arrange — RC config with build.apt_packages = ["make"]; image was built with cmake only.
		t.Chdir(t.TempDir())
		require.NoError(t, os.WriteFile(".agenticrc.toml", []byte("[build]\napt_packages = [\"make\"]\n"), 0o600))

		var capturedOpts tools.BuildOptions
		stubUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0", Apt: "cmake"}, nil)
		stubPruneImages(t, func() (string, error) { return "", nil })

		// Act
		err := runUpdate(updateCmd, []string{"claude"})

		// Assert — image's own apt packages recovered from label, RC config apt not injected
		require.NoError(t, err)
		assert.Equal(t, []string{"cmake"}, capturedOpts.AptPackages)
	})

	t.Run("all flag with explicit base flag applies base to all images", func(t *testing.T) {
		// Arrange — use a temp dir (no RC config) so the only base is the explicit flag.
		t.Chdir(t.TempDir())

		var capturedOpts []tools.BuildOptions
		stubUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = append(capturedOpts, opts)
			return nil
		})
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		stubPruneImages(t, func() (string, error) { return "", nil })
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
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

		// Assert — explicit --base java must reach every target unchanged
		require.NoError(t, err)
		require.Len(t, capturedOpts, 2)
		assert.Equal(t, []string{"java"}, capturedOpts[0].BaseOverride)
		assert.Equal(t, []string{"java"}, capturedOpts[1].BaseOverride)
	})

	t.Run("all flag with base env var applies base to all images", func(t *testing.T) {
		// Arrange — AGENTIC_BASE_OVERRIDE is an explicit env override; it must NOT be cleared.
		t.Chdir(t.TempDir())
		t.Setenv(config.EnvBaseOverride, "java")

		var capturedOpts []tools.BuildOptions
		stubUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = append(capturedOpts, opts)
			return nil
		})
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		stubPruneImages(t, func() (string, error) { return "", nil })
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
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

		// Assert — env var override must reach every target unchanged
		require.NoError(t, err)
		require.Len(t, capturedOpts, 2)
		assert.Equal(t, []string{"java"}, capturedOpts[0].BaseOverride)
		assert.Equal(t, []string{"java"}, capturedOpts[1].BaseOverride)
	})

	t.Run("all flag with tool arg updates only that tool across namespaces", func(t *testing.T) {
		// Arrange
		var updated []string
		stubUpdateTool(t, func(tool, _ string, _ tools.BuildOptions) error {
			updated = append(updated, tool)
			return nil
		})
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		stubPruneImages(t, func() (string, error) { return "", nil })
		stubListAllImages(t, func(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
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
		stubUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = append(capturedOpts, opts)
			return nil
		})
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0"}, nil)
		stubPruneImages(t, func() (string, error) { return "", nil })
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
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

		// Assert — same tool rebuilt across two namespaces should reuse the same
		// CacheBust value, so Docker can serve cached tool-stage layers for the second build
		require.NoError(t, err)
		require.Len(t, capturedOpts, 2)
		assert.NotEmpty(t, capturedOpts[0].CacheBust)
		assert.Equal(t, capturedOpts[0].CacheBust, capturedOpts[1].CacheBust)
	})
}
