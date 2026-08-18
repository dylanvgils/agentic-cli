package update

import (
	"fmt"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDryRun(t *testing.T) {
	t.Run("prints dockerfile skips script", func(t *testing.T) {
		// Arrange
		var scriptCalled bool
		stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error {
			scriptCalled = true
			return nil
		})

		// Act
		out := captureStdout(t, func() {
			err := DryRun("claude", "agentic", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.False(t, scriptCalled)
		assert.Contains(t, out, "FROM")
	})

	t.Run("without tool arg returns error", func(t *testing.T) {
		// Act
		err := DryRun("", "agentic", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--dry-run requires a tool argument")
	})

	t.Run("recovers base from image label", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{Base: "node@24.0.0,java@21.0.1"}, nil)

		// Act
		out := captureStdout(t, func() {
			err := DryRun("claude", "agentic", tools.BuildOptions{Versions: map[string]string{}})
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
			err := DryRun("claude", "agentic", opts)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "temurin")
		assert.NotContains(t, out, "go.dev")
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		err := DryRun("nonexistent", "agentic", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown tool")
	})

	t.Run("inspectImage error is swallowed, falls back to caller opts", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, fmt.Errorf("daemon not running"))
		opts := tools.BuildOptions{BaseOverride: []string{"java"}, Versions: map[string]string{}}

		// Act
		out := captureStdout(t, func() {
			err := DryRun("claude", "agentic", opts)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "temurin")
	})
}

func TestResolve(t *testing.T) {
	t.Run("all scope dispatches to resolveAll", func(t *testing.T) {
		// Arrange
		var capturedFilters []docker.ImageFilter
		stubListAllImages(t, func(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			capturedFilters = filters
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
			}, nil
		})

		// Act
		targets, err := Resolve(Scope{All: true, FilterTool: "claude"}, tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, []docker.ImageFilter{docker.ToolFilter("claude")}, capturedFilters)
	})

	t.Run("scoped resolve dispatches to resolveScoped", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)

		// Act
		targets, err := Resolve(Scope{Names: []string{"claude"}, HasArgs: true, Namespace: "agentic"}, tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, "claude", targets[0].Name)
	})
}

func Test_resolveScoped(t *testing.T) {
	t.Run("single tool always included even if unbuilt", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)

		// Act
		targets, err := resolveScoped([]string{"claude"}, true, "agentic", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, "claude", targets[0].Name)
	})

	t.Run("mixed built recovers opts from label for built tools", func(t *testing.T) {
		// Arrange - first tool not built, remaining tools return this built image
		stubInspectImageSequence(t, nil, &docker.ImageInfo{Version: "1.0.0", Base: "node@24,java@21"})

		// Act
		targets, err := resolveScoped(tools.Names(), false, "agentic", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.NoError(t, err)
		assert.Len(t, targets, len(tools.Names())-1)
		assert.NotEmpty(t, targets[0].Opts.BaseOverride)
	})

	t.Run("inspectImage error propagates", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, fmt.Errorf("daemon not running"))

		// Act
		_, err := resolveScoped([]string{"claude"}, true, "agentic", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.Error(t, err)
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		_, err := resolveScoped([]string{"nonexistent"}, true, "agentic", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown tool")
	})

	t.Run("recently pulled image has its automatic pull throttled", func(t *testing.T) {
		// Arrange
		freshLabel := time.Now().UTC().Format("2006-01-02T15:04:05Z")
		stubInspectImage(t, &docker.ImageInfo{Pulled: freshLabel}, nil)

		// Act - pullExplicit is false, so the fresh label should disable Pull
		targets, err := resolveScoped([]string{"claude"}, true, "agentic", tools.BuildOptions{Pull: true, Versions: map[string]string{}}, false)

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.False(t, targets[0].Opts.Pull)
	})
}

func Test_resolveAll(t *testing.T) {
	t.Run("skips images with empty tool field", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-base", Namespace: "agentic", Tool: ""},
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
			}, nil
		})

		// Act
		targets, err := resolveAll("", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, "claude", targets[0].Name)
	})

	t.Run("skips the proxy image since it is not an updatable tool", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-proxy", Namespace: "agentic", Tool: "proxy"},
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
			}, nil
		})

		// Act
		targets, err := resolveAll("", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, "claude", targets[0].Name)
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
		targets, err := resolveAll("", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert - each target gets its own label-recovered base, not a shared one
		require.NoError(t, err)
		require.Len(t, targets, 2)
		assert.NotEmpty(t, targets[0].Opts.BaseOverride)
		assert.NotEmpty(t, targets[1].Opts.BaseOverride)
		assert.NotEqual(t, targets[0].Opts.BaseOverride, targets[1].Opts.BaseOverride)
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
		targets, err := resolveAll("", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 2)
		assert.Equal(t, []string{"make", "gcc"}, targets[0].Opts.AptPackages)
		assert.Equal(t, []string{"cmake"}, targets[1].Opts.AptPackages)
	})

	t.Run("listAllImages error propagates", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, fmt.Errorf("docker daemon not running")
		})

		// Act
		_, err := resolveAll("", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})

	t.Run("filters to matching tool when provided", func(t *testing.T) {
		// Arrange
		var capturedFilters []docker.ImageFilter
		stubListAllImages(t, func(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			capturedFilters = filters
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
			}, nil
		})

		// Act
		targets, err := resolveAll("claude", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, "claude", targets[0].Name)
		assert.Equal(t, []docker.ImageFilter{docker.ToolFilter("claude")}, capturedFilters)
	})
}

func TestApply(t *testing.T) {
	t.Run("version changed reported", func(t *testing.T) {
		// Arrange
		stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error { return nil })
		stubInspectImageSequence(t, &docker.ImageInfo{Version: "1.0.0"}, &docker.ImageInfo{Version: "2.0.0"})

		// Act
		out := captureStdout(t, func() {
			err := Apply("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})
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
			err := Apply("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})
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
			err := Apply("claude", "agentic-claude", opts)
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
			err := Apply("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})
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
			err := Apply("claude", "agentic-claude", opts)
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
			err := Apply("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})
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
		err := Apply("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})

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

func Test_applyPullThrottle(t *testing.T) {
	freshLabel := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	t.Run("explicit flag leaves opts unchanged even with a fresh label", func(t *testing.T) {
		// Act
		result := applyPullThrottle(tools.BuildOptions{Pull: true}, &docker.ImageInfo{Pulled: freshLabel}, true)

		// Assert
		assert.True(t, result.Pull)
	})

	t.Run("no existing image leaves opts unchanged", func(t *testing.T) {
		// Act
		result := applyPullThrottle(tools.BuildOptions{Pull: true}, nil, false)

		// Assert
		assert.True(t, result.Pull)
	})

	t.Run("fresh pull-last label disables the automatic pull", func(t *testing.T) {
		// Act
		result := applyPullThrottle(tools.BuildOptions{Pull: true}, &docker.ImageInfo{Pulled: freshLabel}, false)

		// Assert
		assert.False(t, result.Pull)
	})

	t.Run("stale pull-last label leaves the automatic pull enabled", func(t *testing.T) {
		// Arrange
		staleLabel := time.Now().Add(-25 * time.Hour).UTC().Format("2006-01-02T15:04:05Z")

		// Act
		result := applyPullThrottle(tools.BuildOptions{Pull: true}, &docker.ImageInfo{Pulled: staleLabel}, false)

		// Assert
		assert.True(t, result.Pull)
	})

	t.Run("empty pull-last label leaves the automatic pull enabled", func(t *testing.T) {
		// Act
		result := applyPullThrottle(tools.BuildOptions{Pull: true}, &docker.ImageInfo{}, false)

		// Assert
		assert.True(t, result.Pull)
	})
}

func TestApplyRecovered(t *testing.T) {
	t.Run("recovers opts from existing image and rebuilds", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{Version: "1.0.0", Base: "node@24,java@21"}, nil)
		var capturedOpts tools.BuildOptions
		stubUpdateTool(t, func(_, _ string, opts tools.BuildOptions) error {
			capturedOpts = opts
			return nil
		})

		// Act
		err := ApplyRecovered("claude", "agentic-claude")

		// Assert
		require.NoError(t, err)
		assert.NotEmpty(t, capturedOpts.BaseOverride)
	})

	t.Run("missing image still rebuilds with empty opts", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)
		var called bool
		stubUpdateTool(t, func(tool, image string, _ tools.BuildOptions) error {
			called = true
			assert.Equal(t, "claude", tool)
			assert.Equal(t, "agentic-claude", image)
			return nil
		})

		// Act
		err := ApplyRecovered("claude", "agentic-claude")

		// Assert
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("update error propagates", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)
		stubUpdateTool(t, func(_, _ string, _ tools.BuildOptions) error { return fmt.Errorf("build failed") })

		// Act
		err := ApplyRecovered("claude", "agentic-claude")

		// Assert
		require.Error(t, err)
	})
}
