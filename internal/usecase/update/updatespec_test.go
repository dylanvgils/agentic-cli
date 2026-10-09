package update

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDryRun(t *testing.T) {
	t.Run("prints dockerfile skips script", func(t *testing.T) {
		// Arrange
		var scriptCalled bool
		d := &fakeDocker{buildTool: func(_, _ string, _ tools.BuildOptions) error {
			scriptCalled = true
			return nil
		}}

		// Act
		out := captureStdout(t, func() {
			err := New(d).DryRun("claude", "agentic", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.False(t, scriptCalled)
		assert.Contains(t, out, "FROM")
	})

	t.Run("without tool arg returns error", func(t *testing.T) {
		// Act
		err := New(&fakeDocker{}).DryRun("", "agentic", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--dry-run requires a tool argument")
	})

	t.Run("recovers base from image label", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Base: "node@24.0.0,java@21.0.1"}, nil)}

		// Act
		out := captureStdout(t, func() {
			err := New(d).DryRun("claude", "agentic", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "temurin")
	})

	t.Run("explicit base flag takes precedence", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Base: "node@24.0.0,go@1.22"}, nil)}
		opts := tools.BuildOptions{BaseOverride: []string{"java"}, Versions: map[string]string{}}

		// Act
		out := captureStdout(t, func() {
			err := New(d).DryRun("claude", "agentic", opts)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "temurin")
		assert.NotContains(t, out, "go.dev")
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		err := New(&fakeDocker{}).DryRun("nonexistent", "agentic", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown tool")
	})

	t.Run("inspectImage error is swallowed, falls back to caller opts", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(nil, fmt.Errorf("daemon not running"))}
		opts := tools.BuildOptions{BaseOverride: []string{"java"}, Versions: map[string]string{}}

		// Act
		out := captureStdout(t, func() {
			err := New(d).DryRun("claude", "agentic", opts)
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
		d := &fakeDocker{listAllImages: func(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			capturedFilters = filters
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
			}, nil
		}}

		// Act
		targets, _, err := New(d).Resolve(Scope{All: true, FilterTool: "claude"}, tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, []docker.ImageFilter{docker.ToolFilter("claude")}, capturedFilters)
	})

	t.Run("scoped resolve dispatches to resolveScoped", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(nil, nil)}

		// Act
		targets, _, err := New(d).Resolve(Scope{Names: []string{"claude"}, HasArgs: true, Namespace: "agentic"}, tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, "claude", targets[0].Name)
	})
}

func Test_resolveScoped(t *testing.T) {
	t.Run("single tool always included even if unbuilt", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(nil, nil)}

		// Act
		targets, _, err := New(d).resolveScoped([]string{"claude"}, true, "agentic", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, "claude", targets[0].Name)
	})

	t.Run("mixed built recovers opts for built tools and reports unbuilt ones as skipped", func(t *testing.T) {
		// Arrange - first tool not built, remaining tools return this built image
		d := &fakeDocker{inspectImage: inspectSequence(nil, &docker.ImageInfo{Version: "1.0.0", Base: "node@24,java@21"})}

		// Act
		targets, skipped, err := New(d).resolveScoped(tools.Names(), false, "agentic", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.NoError(t, err)
		assert.Len(t, targets, len(tools.Names())-1)
		assert.NotEmpty(t, targets[0].Opts.BaseOverride)
		unbuilt, err := tools.ImageName(tools.Names()[0], "agentic")
		require.NoError(t, err)
		assert.Equal(t, []string{unbuilt}, skipped)
	})

	t.Run("inspectImage error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(nil, fmt.Errorf("daemon not running"))}

		// Act
		_, _, err := New(d).resolveScoped([]string{"claude"}, true, "agentic", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.Error(t, err)
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		_, _, err := New(&fakeDocker{}).resolveScoped([]string{"nonexistent"}, true, "agentic", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown tool")
	})

	t.Run("recently pulled image has its automatic pull throttled", func(t *testing.T) {
		// Arrange
		freshLabel := time.Now().UTC().Format("2006-01-02T15:04:05Z")
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Pulled: freshLabel}, nil)}

		// Act - pullExplicit is false, so the fresh label should disable Pull
		targets, _, err := New(d).resolveScoped([]string{"claude"}, true, "agentic", tools.BuildOptions{Pull: true, Versions: map[string]string{}}, false)

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.False(t, targets[0].Opts.Pull)
	})
}

func Test_resolveAll(t *testing.T) {
	t.Run("skips images with empty tool field", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-base", Namespace: "agentic", Tool: ""},
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
			}, nil
		}}

		// Act
		targets, err := New(d).resolveAll("", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, "claude", targets[0].Name)
	})

	t.Run("skips the proxy image since it is not an updatable tool", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-proxy", Namespace: "agentic", Tool: "proxy"},
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
			}, nil
		}}

		// Act
		targets, err := New(d).resolveAll("", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, "claude", targets[0].Name)
	})

	t.Run("recovers base independently from each image label", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude", Base: "java@21"},
				{Image: "work-copilot", Namespace: "work", Tool: "copilot", Base: "dotnet@8"},
			}, nil
		}}

		// Act
		targets, err := New(d).resolveAll("", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert - each target gets its own label-recovered base, not a shared one
		require.NoError(t, err)
		require.Len(t, targets, 2)
		assert.NotEmpty(t, targets[0].Opts.BaseOverride)
		assert.NotEmpty(t, targets[1].Opts.BaseOverride)
		assert.NotEqual(t, targets[0].Opts.BaseOverride, targets[1].Opts.BaseOverride)
	})

	t.Run("recovers apt independently from each image label", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude", Apt: "make,gcc"},
				{Image: "work-copilot", Namespace: "work", Tool: "copilot", Apt: "cmake"},
			}, nil
		}}

		// Act
		targets, err := New(d).resolveAll("", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 2)
		assert.Equal(t, []string{"make", "gcc"}, targets[0].Opts.AptPackages)
		assert.Equal(t, []string{"cmake"}, targets[1].Opts.AptPackages)
	})

	t.Run("listAllImages error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, fmt.Errorf("docker daemon not running")
		}}

		// Act
		_, err := New(d).resolveAll("", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})

	t.Run("filters to matching tool when provided", func(t *testing.T) {
		// Arrange
		var capturedFilters []docker.ImageFilter
		d := &fakeDocker{listAllImages: func(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			capturedFilters = filters
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
			}, nil
		}}

		// Act
		targets, err := New(d).resolveAll("claude", tools.BuildOptions{Versions: map[string]string{}}, true)

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, "claude", targets[0].Name)
		assert.Equal(t, []docker.ImageFilter{docker.ToolFilter("claude")}, capturedFilters)
	})
}

func TestApplyAll(t *testing.T) {
	stubLatestToolVersion(t, "", false, false)
	targets := []Target{
		{Name: "claude", Image: "agentic-claude"},
		{Name: "claude", Image: "work-claude"},
	}

	t.Run("targets share one cache-bust value", func(t *testing.T) {
		// Arrange
		var busts []string
		d := &fakeDocker{buildTool: func(_, _ string, opts tools.BuildOptions) error {
			busts = append(busts, opts.CacheBust)
			return nil
		}}

		// Act
		var err error
		captureLog(t, func() { err = New(d).ApplyAll(targets) })

		// Assert - the same value lets Docker reuse the tool stage for the second namespace
		require.NoError(t, err)
		require.Len(t, busts, 2)
		assert.NotEmpty(t, busts[0])
		assert.Equal(t, busts[0], busts[1])
	})

	t.Run("stops at the first failure", func(t *testing.T) {
		// Arrange
		var built []string
		d := &fakeDocker{buildTool: func(_, image string, _ tools.BuildOptions) error {
			built = append(built, image)
			return errors.New("build failed")
		}}

		// Act
		var err error
		captureLog(t, func() { err = New(d).ApplyAll(targets) })

		// Assert
		require.ErrorContains(t, err, "build failed")
		assert.Equal(t, []string{"agentic-claude"}, built)
	})
}

func TestApply(t *testing.T) {
	stubLatestToolVersion(t, "", false, false)

	t.Run("version changed reported", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil)}
		stubLatestToolVersion(t, "2.0.0", true, true)

		// Act
		out := captureLog(t, func() {
			err := New(d).Apply("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "   version: 1.0.0 -> 2.0.0")
	})

	t.Run("version up to date reported", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil)}
		stubLatestToolVersion(t, "1.0.0", false, true)

		// Act
		out := captureLog(t, func() {
			err := New(d).Apply("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "   version: 1.0.0 (up to date)")
	})

	t.Run("version line hidden when unbuilt", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(nil, nil)}

		// Act
		out := captureLog(t, func() {
			err := New(d).Apply("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.NotContains(t, out, "version:")
	})

	t.Run("version line hidden when check inconclusive", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil)}
		stubLatestToolVersion(t, "", false, false)

		// Act
		out := captureLog(t, func() {
			err := New(d).Apply("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.NotContains(t, out, "version:")
	})

	t.Run("base override shown", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil)}
		opts := tools.BuildOptions{BaseOverride: []string{"java"}, Versions: map[string]string{}}

		// Act
		out := captureLog(t, func() {
			err := New(d).Apply("claude", "agentic-claude", opts)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "   base: java")
	})

	t.Run("base override hidden when empty", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil)}

		// Act
		out := captureLog(t, func() {
			err := New(d).Apply("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.NotContains(t, out, "=> base:")
	})

	t.Run("apt packages shown", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil)}
		opts := tools.BuildOptions{AptPackages: []string{"curl", "jq"}, Versions: map[string]string{}}

		// Act
		out := captureLog(t, func() {
			err := New(d).Apply("claude", "agentic-claude", opts)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "   apt: curl, jq")
	})

	t.Run("apt packages hidden when empty", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil)}

		// Act
		out := captureLog(t, func() {
			err := New(d).Apply("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.NotContains(t, out, "apt:")
	})

	t.Run("empty base-exact reported as none, exact", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil)}
		opts := tools.BuildOptions{BaseExact: true, Versions: map[string]string{}}

		// Act
		out := captureLog(t, func() {
			err := New(d).Apply("claude", "agentic-claude", opts)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "   base: (none, exact)")
	})

	t.Run("empty apt-exact reported as none, exact", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil)}
		opts := tools.BuildOptions{AptExact: true, Versions: map[string]string{}}

		// Act
		out := captureLog(t, func() {
			err := New(d).Apply("claude", "agentic-claude", opts)
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "   apt: (none, exact)")
	})

	t.Run("script error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil),
			buildTool: func(_, _ string, _ tools.BuildOptions) error {
				return fmt.Errorf("docker daemon not running")
			},
		}

		// Act
		err := New(d).Apply("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})

	t.Run("up-to-date image is restamped instead of rebuilt", func(t *testing.T) {
		// Arrange
		stubLatestToolVersion(t, "1.0.0", false, true)
		var restamped, built bool
		d := &fakeDocker{
			inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil),
			restampImage: func(string, docker.ImageInfo) { restamped = true },
			buildTool: func(_, _ string, _ tools.BuildOptions) error {
				built = true
				return nil
			},
		}

		// Act
		err := New(d).Apply("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.NoError(t, err)
		assert.True(t, restamped)
		assert.False(t, built)
	})

	t.Run("newer upstream version rebuilds", func(t *testing.T) {
		// Arrange
		stubLatestToolVersion(t, "2.0.0", true, true)
		var built bool
		d := &fakeDocker{
			inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil),
			buildTool: func(_, _ string, _ tools.BuildOptions) error {
				built = true
				return nil
			},
		}

		// Act
		err := New(d).Apply("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.NoError(t, err)
		assert.True(t, built)
	})

	t.Run("inconclusive upstream check rebuilds", func(t *testing.T) {
		// Arrange
		var built bool
		d := &fakeDocker{
			inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil),
			buildTool: func(_, _ string, _ tools.BuildOptions) error {
				built = true
				return nil
			},
		}

		// Act
		err := New(d).Apply("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.NoError(t, err)
		assert.True(t, built)
	})

	t.Run("inspect error rebuilds without checking upstream", func(t *testing.T) {
		// Arrange
		calls := stubLatestToolVersion(t, "1.0.0", false, true)
		var built bool
		d := &fakeDocker{
			inspectImage: inspectReturns(nil, fmt.Errorf("docker daemon unreachable")),
			buildTool: func(_, _ string, _ tools.BuildOptions) error {
				built = true
				return nil
			},
		}

		// Act
		err := New(d).Apply("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})

		// Assert
		require.NoError(t, err)
		assert.True(t, built)
		assert.Zero(t, calls())
	})

	t.Run("fetches the upstream version once for both the report and the decision", func(t *testing.T) {
		// Arrange
		calls := stubLatestToolVersion(t, "2.0.0", true, true)
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil)}

		// Act
		out := captureLog(t, func() {
			err := New(d).Apply("claude", "agentic-claude", tools.BuildOptions{Versions: map[string]string{}})
			require.NoError(t, err)
		})

		// Assert
		assert.Equal(t, 1, calls())
		assert.Contains(t, out, "   version: 1.0.0 -> 2.0.0")
	})
}

func Test_reportVersionChange(t *testing.T) {
	t.Run("version changed", func(t *testing.T) {
		// Act
		out := captureLog(t, func() { reportVersionChange("1.0.0", "2.0.0") })

		// Assert
		assert.Contains(t, out, "1.0.0 -> 2.0.0")
	})

	t.Run("version up to date", func(t *testing.T) {
		// Act
		out := captureLog(t, func() { reportVersionChange("1.0.0", "1.0.0") })

		// Assert
		assert.Contains(t, out, "(up to date)")
	})

	t.Run("no before version just prints version", func(t *testing.T) {
		// Act
		out := captureLog(t, func() { reportVersionChange("", "1.0.0") })

		// Assert
		assert.Contains(t, out, "1.0.0")
		assert.NotContains(t, out, "(up to date)")
	})

	t.Run("no after version prints nothing", func(t *testing.T) {
		// Act
		out := captureLog(t, func() { reportVersionChange("1.0.0", "") })

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

	t.Run("base-exact skips recovery even when empty", func(t *testing.T) {
		// Act
		result := recoverOpts(&docker.ImageInfo{Base: "node@24,java@21"}, tools.BuildOptions{BaseOverride: []string{}, BaseExact: true})

		// Assert
		assert.Empty(t, result.BaseOverride)
	})

	t.Run("apt-exact skips recovery", func(t *testing.T) {
		// Act
		result := recoverOpts(&docker.ImageInfo{Base: "node@24", Apt: "make,gcc"}, tools.BuildOptions{AptPackages: []string{"cmake"}, AptExact: true})

		// Assert
		assert.Equal(t, []string{"cmake"}, result.AptPackages)
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
	stubLatestToolVersion(t, "", false, false)

	t.Run("recovers opts from existing image and rebuilds", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		d := &fakeDocker{
			inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0", Base: "node@24,java@21"}, nil),
			buildTool: func(_, _ string, opts tools.BuildOptions) error {
				capturedOpts = opts
				return nil
			},
		}

		// Act
		err := New(d).ApplyRecovered("claude", "agentic-claude", &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.NotEmpty(t, capturedOpts.BaseOverride)
	})

	t.Run("custom installs come from rc, not label recovery", func(t *testing.T) {
		// Arrange
		var capturedOpts tools.BuildOptions
		d := &fakeDocker{
			inspectImage: inspectReturns(&docker.ImageInfo{Version: "1.0.0"}, nil),
			buildTool: func(_, _ string, opts tools.BuildOptions) error {
				capturedOpts = opts
				return nil
			},
		}
		rc := &config.AgenticRC{Build: config.RCBuild{CustomInstalls: []config.RCCustomInstall{{Name: "helm", Run: []string{"true"}}}}}

		// Act
		err := New(d).ApplyRecovered("claude", "agentic-claude", rc)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, rc.Build.CustomInstalls, capturedOpts.CustomInstalls)
	})

	t.Run("missing image still rebuilds with empty opts", func(t *testing.T) {
		// Arrange
		var called bool
		d := &fakeDocker{
			buildTool: func(tool, image string, _ tools.BuildOptions) error {
				called = true
				assert.Equal(t, "claude", tool)
				assert.Equal(t, "agentic-claude", image)
				return nil
			},
		}

		// Act
		err := New(d).ApplyRecovered("claude", "agentic-claude", &config.AgenticRC{})

		// Assert
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("update error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			buildTool: func(_, _ string, _ tools.BuildOptions) error { return fmt.Errorf("build failed") },
		}

		// Act
		err := New(d).ApplyRecovered("claude", "agentic-claude", &config.AgenticRC{})

		// Assert
		require.Error(t, err)
	})
}

func Test_rebuild(t *testing.T) {
	captureOpts := func(opts *tools.BuildOptions, built *bool) *fakeDocker {
		return &fakeDocker{buildTool: func(_, _ string, o tools.BuildOptions) error {
			*opts = o
			*built = true
			return nil
		}}
	}

	t.Run("recovers base from label", func(t *testing.T) {
		// Arrange
		var opts tools.BuildOptions
		var built bool
		info := &docker.ImageInfo{Base: "node@24.0.0,java@21.0.1"}

		// Act
		err := New(captureOpts(&opts, &built)).rebuild("claude", "agentic-claude", info, false, tools.BuildOptions{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, opts.BaseOverride, "java")
	})

	t.Run("respects existing base override", func(t *testing.T) {
		// Arrange
		var opts tools.BuildOptions
		var built bool
		info := &docker.ImageInfo{Base: "node@24.0.0,dotnet@8.0"}

		// Act
		err := New(captureOpts(&opts, &built)).rebuild("claude", "agentic-claude", info, false, tools.BuildOptions{BaseOverride: []string{"java"}})

		// Assert - explicit BaseOverride wins over the dotnet recovered from the label
		require.NoError(t, err)
		assert.Equal(t, []string{"java"}, opts.BaseOverride)
	})

	t.Run("base-exact skips label recovery even when override is empty", func(t *testing.T) {
		// Arrange
		var opts tools.BuildOptions
		var built bool
		info := &docker.ImageInfo{Base: "node@24.0.0,java@21.0.1"}

		// Act
		err := New(captureOpts(&opts, &built)).rebuild("claude", "agentic-claude", info, false, tools.BuildOptions{BaseOverride: []string{}, BaseExact: true})

		// Assert
		require.NoError(t, err)
		assert.Empty(t, opts.BaseOverride)
	})

	t.Run("recovers layer versions from label", func(t *testing.T) {
		// Arrange
		var opts tools.BuildOptions
		var built bool
		info := &docker.ImageInfo{Base: "node@24.0.0,java@21.0.1", VersionArgs: "node@24,java@17"}

		// Act - no --java flag passed, so the recovered version must be the one used
		err := New(captureOpts(&opts, &built)).rebuild("claude", "agentic-claude", info, false, tools.BuildOptions{})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "17", opts.Versions["java"])
	})

	t.Run("user-provided version flag wins over recovered label", func(t *testing.T) {
		// Arrange
		var opts tools.BuildOptions
		var built bool
		info := &docker.ImageInfo{Base: "node@24.0.0,java@21.0.1", VersionArgs: "node@24,java@17"}

		// Act
		err := New(captureOpts(&opts, &built)).rebuild("claude", "agentic-claude", info, false, tools.BuildOptions{Versions: map[string]string{"java": "21"}})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "21", opts.Versions["java"])
	})

	t.Run("recovers apt packages from label", func(t *testing.T) {
		// Arrange
		var opts tools.BuildOptions
		var built bool
		info := &docker.ImageInfo{Apt: "make,gcc"}

		// Act
		err := New(captureOpts(&opts, &built)).rebuild("claude", "agentic-claude", info, false, tools.BuildOptions{})

		// Assert
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"make", "gcc"}, opts.AptPackages)
	})

	t.Run("merges label apt packages with user-provided packages", func(t *testing.T) {
		// Arrange
		var opts tools.BuildOptions
		var built bool
		info := &docker.ImageInfo{Apt: "make"}

		// Act
		err := New(captureOpts(&opts, &built)).rebuild("claude", "agentic-claude", info, false, tools.BuildOptions{AptPackages: []string{"gcc"}})

		// Assert
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"make", "gcc"}, opts.AptPackages)
	})

	t.Run("apt-exact skips label apt recovery", func(t *testing.T) {
		// Arrange
		var opts tools.BuildOptions
		var built bool
		info := &docker.ImageInfo{Apt: "make"}

		// Act
		err := New(captureOpts(&opts, &built)).rebuild("claude", "agentic-claude", info, false, tools.BuildOptions{AptPackages: []string{"gcc"}, AptExact: true})

		// Assert - the recovered "make" package does not survive, only the exact "gcc" does
		require.NoError(t, err)
		assert.Equal(t, []string{"gcc"}, opts.AptPackages)
	})

	t.Run("skips verification when all user packages already in image", func(t *testing.T) {
		// Arrange
		var opts tools.BuildOptions
		var built bool
		info := &docker.ImageInfo{Apt: "make,gcc"}

		// Act
		err := New(captureOpts(&opts, &built)).rebuild("claude", "agentic-claude", info, false, tools.BuildOptions{AptPackages: []string{"make"}})

		// Assert
		require.NoError(t, err)
		assert.False(t, opts.VerifyApt)
	})

	t.Run("verifies when user provides package not in image", func(t *testing.T) {
		// Arrange
		var opts tools.BuildOptions
		var built bool
		info := &docker.ImageInfo{Apt: "make"}

		// Act
		err := New(captureOpts(&opts, &built)).rebuild("claude", "agentic-claude", info, false, tools.BuildOptions{AptPackages: []string{"gcc"}})

		// Assert
		require.NoError(t, err)
		assert.True(t, opts.VerifyApt)
	})

	t.Run("up to date with pull false restamps instead of rebuilding", func(t *testing.T) {
		// Arrange
		var built bool
		var restamped *docker.ImageInfo
		d := &fakeDocker{
			buildTool: func(_, _ string, _ tools.BuildOptions) error {
				built = true
				return nil
			},
			restampImage: func(_ string, info docker.ImageInfo) { restamped = &info },
		}
		info := &docker.ImageInfo{Version: "1.2.3", Apt: "make,gcc"}

		// Act
		err := New(d).rebuild("claude", "agentic-claude", info, true, tools.BuildOptions{})

		// Assert
		require.NoError(t, err)
		assert.False(t, built, "up-to-date path must not invoke a full docker build")
		require.NotNil(t, restamped)
		assert.Equal(t, "make,gcc", restamped.Apt, "unchanged labels must be carried forward")
	})

	t.Run("no-cache bypasses the up-to-date check", func(t *testing.T) {
		// Arrange
		var opts tools.BuildOptions
		var built bool

		// Act
		err := New(captureOpts(&opts, &built)).rebuild("claude", "agentic-claude", &docker.ImageInfo{Version: "1.2.3"}, true, tools.BuildOptions{NoCache: true})

		// Assert
		require.NoError(t, err)
		assert.True(t, built)
	})

	t.Run("pull bypasses the up-to-date check so base layers still get refreshed", func(t *testing.T) {
		// Arrange
		var opts tools.BuildOptions
		var built bool

		// Act
		err := New(captureOpts(&opts, &built)).rebuild("claude", "agentic-claude", &docker.ImageInfo{Version: "1.2.3"}, true, tools.BuildOptions{Pull: true})

		// Assert
		require.NoError(t, err)
		assert.True(t, built)
	})

	t.Run("pull-only rebuild of an up-to-date tool does not bust the tool stage cache", func(t *testing.T) {
		// Arrange
		var opts tools.BuildOptions
		var built bool

		// Act
		err := New(captureOpts(&opts, &built)).rebuild("claude", "agentic-claude", &docker.ImageInfo{Version: "1.2.3"}, true, tools.BuildOptions{Pull: true})

		// Assert - an empty CacheBust means no CACHEBUST build arg, so the tool stage isn't reinstalled
		require.NoError(t, err)
		assert.Empty(t, opts.CacheBust)
	})

	t.Run("pull-only rebuild reuses the recorded cachebust instead of falling back to a stale build", func(t *testing.T) {
		// Arrange
		var opts tools.BuildOptions
		var built bool
		info := &docker.ImageInfo{Version: "1.2.3", CacheBust: "2026-08-21T07:18:37Z"}

		// Act
		err := New(captureOpts(&opts, &built)).rebuild("claude", "agentic-claude", info, true, tools.BuildOptions{Pull: true})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "2026-08-21T07:18:37Z", opts.CacheBust)
	})

	t.Run("not up to date always sets a cachebust", func(t *testing.T) {
		// Arrange
		var opts tools.BuildOptions
		var built bool

		// Act
		err := New(captureOpts(&opts, &built)).rebuild("claude", "agentic-claude", nil, false, tools.BuildOptions{})

		// Assert - a non-empty CacheBust makes the tool build skip cache via --build-arg=CACHEBUST=<value>
		require.NoError(t, err)
		assert.True(t, built)
		assert.NotEmpty(t, opts.CacheBust)
	})
}

func Test_mergeVersions(t *testing.T) {
	t.Run("overrides win over recovered", func(t *testing.T) {
		// Act
		result := mergeVersions(map[string]string{"node": "24", "java": "17"}, map[string]string{"java": "21"})

		// Assert
		assert.Equal(t, map[string]string{"node": "24", "java": "21"}, result)
	})

	t.Run("empty override values are ignored", func(t *testing.T) {
		// Act
		result := mergeVersions(map[string]string{"node": "24"}, map[string]string{"node": "", "java": "17"})

		// Assert
		assert.Equal(t, map[string]string{"node": "24", "java": "17"}, result)
	})

	t.Run("no recovered values returns overrides", func(t *testing.T) {
		// Act
		result := mergeVersions(nil, map[string]string{"java": "17"})

		// Assert
		assert.Equal(t, map[string]string{"java": "17"}, result)
	})
}

func Test_hasNewAptPackages(t *testing.T) {
	t.Run("all packages in existing returns false", func(t *testing.T) {
		// Act
		result := hasNewAptPackages([]string{"make", "gcc"}, []string{"make", "gcc", "jq"})

		// Assert
		assert.False(t, result)
	})

	t.Run("new package returns true", func(t *testing.T) {
		// Act
		result := hasNewAptPackages([]string{"make", "curl"}, []string{"make"})

		// Assert
		assert.True(t, result)
	})

	t.Run("empty requested returns false", func(t *testing.T) {
		// Act
		result := hasNewAptPackages(nil, []string{"make"})

		// Assert
		assert.False(t, result)
	})
}
