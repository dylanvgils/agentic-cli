package update

import (
	"errors"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDryRun(t *testing.T) {
	t.Run("prints the labelled dockerfile without building", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{}
		built := stubBuilds(d, nil)
		errOut := iostreamstest.StubErr(t)

		// Act
		var err error
		out := iostreamstest.CaptureOut(t, func() { err = newTestService(t, d, &fakeUpstream{}).DryRun("claude", "agentic", tools.BuildOptions{}) })

		// Assert
		require.NoError(t, err)
		assert.Contains(t, out, "FROM")
		assert.Equal(t, "agentic: claude Dockerfile:\n\n", errOut.String())
		assert.Empty(t, built.images)
	})

	t.Run("recovers build options from the built image", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Base: "node@24.0.0,java@21.0.1"}, nil)}

		// Act
		var err error
		out := iostreamstest.CaptureOut(t, func() { err = newTestService(t, d, &fakeUpstream{}).DryRun("claude", "agentic", tools.BuildOptions{}) })

		// Assert
		require.NoError(t, err)
		assert.Contains(t, out, "temurin")
	})

	t.Run("inspect error falls back to the caller's options", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(nil, errors.New("daemon not running"))}

		// Act
		var err error
		out := iostreamstest.CaptureOut(t, func() {
			err = newTestService(t, d, &fakeUpstream{}).DryRun("claude", "agentic", tools.BuildOptions{BaseOverride: []string{"java"}})
		})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, out, "temurin")
	})

	for _, tc := range []struct {
		name    string
		tool    string
		wantErr string
	}{
		{name: "without a tool", tool: "", wantErr: "--dry-run requires a tool argument"},
		{name: "unknown tool", tool: "nonexistent", wantErr: "unknown tool"},
	} {
		t.Run(tc.name+" returns an error", func(t *testing.T) {
			// Act
			err := newTestService(t, &fakeDocker{}, &fakeUpstream{}).DryRun(tc.tool, "agentic", tools.BuildOptions{})

			// Assert
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestResolve(t *testing.T) {
	t.Run("all scope resolves every image of the tool", func(t *testing.T) {
		// Arrange
		var capturedFilters []docker.ImageFilter
		d := &fakeDocker{listAllImages: func(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			capturedFilters = filters
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
			}, nil
		}}

		// Act
		targets, err := newTestService(t, d, &fakeUpstream{}).Resolve(Scope{All: true, Tool: "claude"}, tools.BuildOptions{}, true)

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, []docker.ImageFilter{docker.ToolFilter("claude")}, capturedFilters)
	})

	t.Run("single tool scope resolves that tool", func(t *testing.T) {
		// Act
		targets, err := newTestService(t, &fakeDocker{}, &fakeUpstream{}).Resolve(Scope{Tool: "claude", Namespace: "agentic"}, tools.BuildOptions{}, true)

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, "claude", targets[0].Name)
	})
}

func Test_resolveTool(t *testing.T) {
	t.Run("unbuilt tool is included", func(t *testing.T) {
		// Act
		target, err := newTestService(t, &fakeDocker{}, &fakeUpstream{}).resolveTool("claude", "agentic", tools.BuildOptions{}, true)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, Target{Name: "claude", Image: "agentic-claude"}, target)
	})

	t.Run("built tool recovers its build options and throttles a recent pull", func(t *testing.T) {
		// Arrange
		fresh := time.Now().UTC().Format(time.RFC3339)
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{Apt: "make", Pulled: fresh}, nil)}

		// Act
		target, err := newTestService(t, d, &fakeUpstream{}).resolveTool("claude", "agentic", tools.BuildOptions{Pull: true}, false)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"make"}, target.Opts.AptPackages)
		assert.False(t, target.Opts.Pull)
	})

	t.Run("inspect error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(nil, errors.New("daemon not running"))}

		// Act
		_, err := newTestService(t, d, &fakeUpstream{}).resolveTool("claude", "agentic", tools.BuildOptions{}, true)

		// Assert
		assert.ErrorContains(t, err, "daemon not running")
	})

	t.Run("unknown tool returns an error", func(t *testing.T) {
		// Act
		_, err := newTestService(t, &fakeDocker{}, &fakeUpstream{}).resolveTool("nonexistent", "agentic", tools.BuildOptions{}, true)

		// Assert
		assert.ErrorContains(t, err, "unknown tool")
	})
}

func Test_resolveAll(t *testing.T) {
	t.Run("skips images that are not updatable tools", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-base", Namespace: "agentic", Tool: ""},
				{Image: "agentic-proxy", Namespace: "agentic", Tool: "proxy"},
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
			}, nil
		}}

		// Act
		targets, err := newTestService(t, d, &fakeUpstream{}).resolveAll("", tools.BuildOptions{}, true)

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, "agentic-claude", targets[0].Image)
	})

	t.Run("each image keeps its own recovered options", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		helmHash := storeCustomInstalls(t, home, config.RCCustomInstall{Name: "helm", Run: []string{"echo helm"}})
		d := &fakeDocker{listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude", Apt: "make,gcc"},
				{Image: "work-claude", Namespace: "work", Tool: "claude", Apt: "cmake", CustomInstalls: "helm", CustomInstallsHash: helmHash},
			}, nil
		}}
		opts := tools.BuildOptions{CustomInstalls: []config.RCCustomInstall{{Name: "golangci-lint", Run: []string{"echo lint"}}}}

		// Act
		targets, err := New(d, &fakeUpstream{}, home).resolveAll("", opts, true)

		// Assert - the cwd config's golangci-lint isn't added to either image
		require.NoError(t, err)
		require.Len(t, targets, 2)
		assert.Equal(t, []string{"make", "gcc"}, targets[0].Opts.AptPackages)
		assert.Empty(t, targets[0].Opts.CustomInstalls)
		assert.Equal(t, []string{"cmake"}, targets[1].Opts.AptPackages)
		assert.Equal(t, []config.RCCustomInstall{{Name: "helm", Run: []string{"echo helm"}}}, targets[1].Opts.CustomInstalls)
	})

	t.Run("list error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, errors.New("docker daemon not running")
		}}

		// Act
		_, err := newTestService(t, d, &fakeUpstream{}).resolveAll("", tools.BuildOptions{}, true)

		// Assert
		assert.ErrorContains(t, err, "docker daemon not running")
	})
}

func TestApplyAll(t *testing.T) {
	targets := []Target{
		{Name: "claude", Image: "agentic-claude"},
		{Name: "claude", Image: "work-claude"},
	}

	t.Run("every target builds with one cache bust, then prunes and records the check", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		pruned := false
		d := &fakeDocker{pruneImages: func() (string, error) { pruned = true; return "", nil }}
		built := stubBuilds(d, nil)

		// Act
		var err error
		iostreamstest.CaptureErr(t, func() { err = New(d, &fakeUpstream{}, home).ApplyAll(targets) })

		// Assert - one value lets Docker reuse the tool stage for the second namespace
		require.NoError(t, err)
		require.Len(t, built.opts, 2)
		assert.NotEmpty(t, built.opts[0].CacheBust)
		assert.Equal(t, built.opts[0].CacheBust, built.opts[1].CacheBust)
		assert.True(t, pruned)
		state, err := config.LoadState(home)
		require.NoError(t, err)
		assert.Contains(t, state.LastToolVersionCheck, "claude")
	})

	t.Run("a failed target stops the rest, skips the prune and records no check", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		pruned := false
		d := &fakeDocker{pruneImages: func() (string, error) { pruned = true; return "", nil }}
		built := stubBuilds(d, errors.New("build failed"))

		// Act
		var err error
		iostreamstest.CaptureErr(t, func() { err = New(d, &fakeUpstream{}, home).ApplyAll(targets) })

		// Assert
		require.ErrorContains(t, err, "build failed")
		assert.Equal(t, []string{"agentic-claude"}, built.images)
		assert.False(t, pruned)
		state, err := config.LoadState(home)
		require.NoError(t, err)
		assert.NotContains(t, state.LastToolVersionCheck, "claude")
	})
}

func TestApply(t *testing.T) {
	built := &docker.ImageInfo{Version: "1.0.0"}

	for _, tc := range []struct {
		name     string
		info     *docker.ImageInfo
		upstream fakeUpstream
		opts     tools.BuildOptions
		want     string
	}{
		{
			name: "nothing but the image for an unbuilt tool",
			want: "=> agentic-claude\n",
		},
		{
			name:     "a newer upstream version",
			info:     built,
			upstream: fakeUpstream{latest: "2.0.0", newer: true, ok: true},
			want:     "=> agentic-claude\n   version: 1.0.0 -> 2.0.0\n",
		},
		{
			name:     "an up-to-date version",
			info:     built,
			upstream: fakeUpstream{latest: "1.0.0", ok: true},
			want:     "=> agentic-claude\n   version: 1.0.0 (up to date)\n",
		},
		{
			name: "no version line when the upstream check is inconclusive",
			info: built,
			want: "=> agentic-claude\n",
		},
		{
			name: "base and apt overrides",
			opts: tools.BuildOptions{BaseOverride: []string{"java"}, AptPackages: []string{"curl", "jq"}},
			want: "=> agentic-claude\n   base: java\n   apt: curl, jq\n",
		},
		{
			name: "empty exact base and apt",
			opts: tools.BuildOptions{BaseExact: true, AptExact: true},
			want: "=> agentic-claude\n   base: (none, exact)\n   apt: (none, exact)\n",
		},
	} {
		t.Run("reports "+tc.name, func(t *testing.T) {
			// Arrange
			d := &fakeDocker{inspectImage: inspectReturns(tc.info, nil)}

			// Act
			var err error
			out := iostreamstest.CaptureErr(t, func() { err = newTestService(t, d, &tc.upstream).Apply("claude", "agentic-claude", tc.opts) })

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, out)
		})
	}

	for _, tc := range []struct {
		name          string
		inspectErr    error
		upstream      fakeUpstream
		wantRestamp   bool
		wantUpstreams int
	}{
		{
			name:          "up-to-date image is restamped instead of rebuilt",
			upstream:      fakeUpstream{latest: "1.0.0", ok: true},
			wantRestamp:   true,
			wantUpstreams: 1,
		},
		{
			name:          "newer upstream version rebuilds",
			upstream:      fakeUpstream{latest: "2.0.0", newer: true, ok: true},
			wantUpstreams: 1,
		},
		{
			name:          "inconclusive upstream check rebuilds",
			wantUpstreams: 1,
		},
		{
			name:       "inspect error rebuilds without checking upstream",
			inspectErr: errors.New("docker daemon unreachable"),
			upstream:   fakeUpstream{latest: "1.0.0", ok: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			restamped := false
			d := &fakeDocker{
				inspectImage: inspectReturns(built, tc.inspectErr),
				restampImage: func(string, docker.ImageInfo) { restamped = true },
			}
			builds := stubBuilds(d, nil)

			// Act
			var err error
			iostreamstest.CaptureErr(t, func() { err = newTestService(t, d, &tc.upstream).Apply("claude", "agentic-claude", tools.BuildOptions{}) })

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.wantRestamp, restamped)
			assert.Equal(t, !tc.wantRestamp, len(builds.images) == 1)
			assert.Equal(t, tc.wantUpstreams, tc.upstream.calls)
		})
	}

	t.Run("build error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{inspectImage: inspectReturns(built, nil)}
		stubBuilds(d, errors.New("docker daemon not running"))

		// Act
		var err error
		iostreamstest.CaptureErr(t, func() { err = newTestService(t, d, &fakeUpstream{}).Apply("claude", "agentic-claude", tools.BuildOptions{}) })

		// Assert
		assert.ErrorContains(t, err, "docker daemon not running")
	})
}

func TestApplyRecovered(t *testing.T) {
	rc := &config.AgenticRC{Build: config.RCBuild{CustomInstalls: []config.RCCustomInstall{{Name: "helm", Run: []string{"true"}}}}}

	t.Run("custom installs come from the built image, not rc", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		lintHash := storeCustomInstalls(t, home, config.RCCustomInstall{Name: "golangci-lint", Run: []string{"echo lint"}})
		d := &fakeDocker{inspectImage: inspectReturns(&docker.ImageInfo{CustomInstalls: "golangci-lint", CustomInstallsHash: lintHash}, nil)}
		built := stubBuilds(d, nil)

		// Act
		var err error
		iostreamstest.CaptureErr(t, func() { err = New(d, &fakeUpstream{}, home).ApplyRecovered("claude", "agentic-claude", rc) })

		// Assert
		require.NoError(t, err)
		require.Len(t, built.opts, 1)
		assert.Equal(t, []config.RCCustomInstall{{Name: "golangci-lint", Run: []string{"echo lint"}}}, built.opts[0].CustomInstalls)
	})

	t.Run("missing image builds with the rc's custom installs", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{}
		built := stubBuilds(d, nil)

		// Act
		var err error
		iostreamstest.CaptureErr(t, func() { err = newTestService(t, d, &fakeUpstream{}).ApplyRecovered("claude", "agentic-claude", rc) })

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"agentic-claude"}, built.images)
		assert.Equal(t, rc.Build.CustomInstalls, built.opts[0].CustomInstalls)
	})

	t.Run("build error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{}
		stubBuilds(d, errors.New("build failed"))

		// Act
		var err error
		iostreamstest.CaptureErr(t, func() { err = newTestService(t, d, &fakeUpstream{}).ApplyRecovered("claude", "agentic-claude", rc) })

		// Assert
		assert.ErrorContains(t, err, "build failed")
	})
}

func Test_rebuild(t *testing.T) {
	t.Run("stores custom installs before building", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		installs := []config.RCCustomInstall{{Name: "helm", Run: []string{"echo helm"}}}

		// Act
		err := New(&fakeDocker{}, &fakeUpstream{}, home).rebuild("claude", "agentic-claude", nil, false, tools.BuildOptions{CustomInstalls: installs})

		// Assert
		require.NoError(t, err)
		stored, ok := config.LoadCustomInstalls(home, config.CustomInstallsHash(installs))
		assert.True(t, ok)
		assert.Equal(t, installs, stored)
	})

	t.Run("recovers base, layer versions and apt packages from the labels", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{}
		built := stubBuilds(d, nil)
		info := &docker.ImageInfo{
			Base:        "node@24.0.0,java@21.0.1",
			VersionArgs: "node@24,java@17",
			Apt:         "make",
		}

		// Act
		err := newTestService(t, d, &fakeUpstream{}).rebuild("claude", "agentic-claude", info, false, tools.BuildOptions{})

		// Assert
		require.NoError(t, err)
		require.Len(t, built.opts, 1)
		assert.Contains(t, built.opts[0].BaseOverride, "java")
		assert.Equal(t, "17", built.opts[0].Versions["java"])
		assert.Equal(t, []string{"make"}, built.opts[0].AptPackages)
	})

	for _, tc := range []struct {
		name       string
		info       *docker.ImageInfo
		userApt    []string
		wantVerify bool
	}{
		{name: "skips apt verification without user packages", info: &docker.ImageInfo{Apt: "make"}},
		{name: "skips apt verification when the image has every user package", info: &docker.ImageInfo{Apt: "make,gcc"}, userApt: []string{"make"}},
		{name: "verifies apt when a user package is new to the image", info: &docker.ImageInfo{Apt: "make"}, userApt: []string{"gcc"}, wantVerify: true},
		{name: "verifies apt for a user package on an unbuilt image", userApt: []string{"gcc"}, wantVerify: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			d := &fakeDocker{}
			built := stubBuilds(d, nil)

			// Act
			err := newTestService(t, d, &fakeUpstream{}).rebuild("claude", "agentic-claude", tc.info, false, tools.BuildOptions{AptPackages: tc.userApt})

			// Assert
			require.NoError(t, err)
			require.Len(t, built.opts, 1)
			assert.Equal(t, tc.wantVerify, built.opts[0].VerifyApt)
		})
	}

	t.Run("up-to-date image is restamped with its labels instead of rebuilt", func(t *testing.T) {
		// Arrange
		var restamped *docker.ImageInfo
		d := &fakeDocker{restampImage: func(_ string, info docker.ImageInfo) { restamped = &info }}
		built := stubBuilds(d, nil)
		info := &docker.ImageInfo{Version: "1.2.3", Apt: "make,gcc"}

		// Act
		err := newTestService(t, d, &fakeUpstream{}).rebuild("claude", "agentic-claude", info, true, tools.BuildOptions{})

		// Assert
		require.NoError(t, err)
		assert.Empty(t, built.images)
		require.NotNil(t, restamped)
		assert.Equal(t, *info, *restamped)
	})

	for _, tc := range []struct {
		name      string
		info      *docker.ImageInfo
		upToDate  bool
		opts      tools.BuildOptions
		wantBust  string
		freshBust bool
	}{
		{
			name:      "not up to date builds with a fresh cache bust",
			freshBust: true,
		},
		{
			name:      "no-cache rebuilds an up-to-date image with a fresh cache bust",
			info:      &docker.ImageInfo{Version: "1.2.3"},
			upToDate:  true,
			opts:      tools.BuildOptions{NoCache: true},
			freshBust: true,
		},
		{
			name:     "pull rebuilds an up-to-date image without busting the tool stage",
			info:     &docker.ImageInfo{Version: "1.2.3"},
			upToDate: true,
			opts:     tools.BuildOptions{Pull: true},
		},
		{
			name:     "pull rebuilds an up-to-date image with its recorded cache bust",
			info:     &docker.ImageInfo{Version: "1.2.3", CacheBust: "2026-08-21T07:18:37Z"},
			upToDate: true,
			opts:     tools.BuildOptions{Pull: true},
			wantBust: "2026-08-21T07:18:37Z",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			d := &fakeDocker{}
			built := stubBuilds(d, nil)

			// Act
			err := newTestService(t, d, &fakeUpstream{}).rebuild("claude", "agentic-claude", tc.info, tc.upToDate, tc.opts)

			// Assert
			require.NoError(t, err)
			require.Len(t, built.opts, 1)
			if tc.freshBust {
				assert.NotEmpty(t, built.opts[0].CacheBust)
			} else {
				assert.Equal(t, tc.wantBust, built.opts[0].CacheBust)
			}
		})
	}
}

func Test_recoverOpts(t *testing.T) {
	// Arrange
	home := t.TempDir()
	hash := storeCustomInstalls(t, home, config.RCCustomInstall{Name: "helm", Run: []string{"echo helm"}})
	info := &docker.ImageInfo{
		Base:               "node@24,java@21",
		Apt:                "make",
		CustomInstalls:     "helm",
		CustomInstallsHash: hash,
	}

	// Act
	result := recoverOpts(home, info, tools.BuildOptions{AptPackages: []string{"cmake"}})

	// Assert
	assert.Contains(t, result.BaseOverride, "java")
	assert.Equal(t, []string{"make", "cmake"}, result.AptPackages)
	assert.Equal(t, []config.RCCustomInstall{{Name: "helm", Run: []string{"echo helm"}}}, result.CustomInstalls)
}

func Test_recoverCustomInstalls(t *testing.T) {
	configHelm := config.RCCustomInstall{Name: "helm", Run: []string{"echo config"}}

	t.Run("stored installs win over config", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		hash := storeCustomInstalls(t, home, config.RCCustomInstall{Name: "helm", Run: []string{"echo helm"}})

		// Act
		installs := recoverCustomInstalls(home, &docker.ImageInfo{CustomInstalls: "helm", CustomInstallsHash: hash}, []config.RCCustomInstall{configHelm})

		// Assert
		assert.Equal(t, []config.RCCustomInstall{{Name: "helm", Run: []string{"echo helm"}}}, installs)
	})

	t.Run("image without custom installs ignores config", func(t *testing.T) {
		// Act
		installs := recoverCustomInstalls(t.TempDir(), &docker.ImageInfo{}, []config.RCCustomInstall{configHelm})

		// Assert
		assert.Empty(t, installs)
	})

	t.Run("unstored installs are looked up by name in config", func(t *testing.T) {
		// Arrange
		info := &docker.ImageInfo{Image: "agentic-claude", CustomInstalls: "helm", CustomInstallsHash: config.CustomInstallsHash([]config.RCCustomInstall{configHelm})}

		// Act
		installs := recoverCustomInstalls(t.TempDir(), info, []config.RCCustomInstall{{Name: "kubectl"}, configHelm})

		// Assert
		assert.Equal(t, []config.RCCustomInstall{configHelm}, installs)
	})

	t.Run("names missing from config are dropped with a warning", func(t *testing.T) {
		// Arrange
		errLog := iostreamstest.StubErr(t)
		info := &docker.ImageInfo{Image: "agentic-claude", CustomInstalls: "helm,terraform"}

		// Act
		installs := recoverCustomInstalls(t.TempDir(), info, []config.RCCustomInstall{configHelm})

		// Assert
		assert.Equal(t, []config.RCCustomInstall{configHelm}, installs)
		assert.Contains(t, errLog.String(), "agentic-claude: custom installs terraform not found")
	})
}

func Test_applyPullThrottle(t *testing.T) {
	fresh := &docker.ImageInfo{Pulled: time.Now().UTC().Format(time.RFC3339)}
	stale := &docker.ImageInfo{Pulled: time.Now().Add(-25 * time.Hour).UTC().Format(time.RFC3339)}

	for _, tc := range []struct {
		name     string
		info     *docker.ImageInfo
		explicit bool
		wantPull bool
	}{
		{name: "explicit pull is kept even after a recent pull", info: fresh, explicit: true, wantPull: true},
		{name: "automatic pull is kept without an image", wantPull: true},
		{name: "automatic pull is kept after a stale pull", info: stale, wantPull: true},
		{name: "automatic pull is dropped after a recent pull", info: fresh},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := applyPullThrottle(tools.BuildOptions{Pull: true}, tc.info, tc.explicit)

			// Assert
			assert.Equal(t, tc.wantPull, result.Pull)
		})
	}
}

func Test_reportVersionChange(t *testing.T) {
	for _, tc := range []struct {
		name   string
		before string
		after  string
		want   string
	}{
		{name: "changed version", before: "1.0.0", after: "2.0.0", want: "   version: 1.0.0 -> 2.0.0\n"},
		{name: "same version", before: "1.0.0", after: "1.0.0", want: "   version: 1.0.0 (up to date)\n"},
		{name: "no before version", after: "1.0.0", want: "   version: 1.0.0\n"},
		{name: "no after version", before: "1.0.0", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			out := iostreamstest.CaptureErr(t, func() { reportVersionChange(tc.before, tc.after) })

			// Assert
			assert.Equal(t, tc.want, out)
		})
	}
}

func Test_mergeVersions(t *testing.T) {
	for _, tc := range []struct {
		name      string
		recovered map[string]string
		overrides map[string]string
		want      map[string]string
	}{
		{
			name:      "overrides win over recovered",
			recovered: map[string]string{"node": "24", "java": "17"},
			overrides: map[string]string{"java": "21"},
			want:      map[string]string{"node": "24", "java": "21"},
		},
		{
			name:      "empty override values are ignored",
			recovered: map[string]string{"node": "24"},
			overrides: map[string]string{"node": "", "java": "17"},
			want:      map[string]string{"node": "24", "java": "17"},
		},
		{
			name:      "no recovered values returns overrides",
			overrides: map[string]string{"java": "17"},
			want:      map[string]string{"java": "17"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := mergeVersions(tc.recovered, tc.overrides)

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}

func Test_hasNewAptPackages(t *testing.T) {
	for _, tc := range []struct {
		name      string
		requested []string
		want      bool
	}{
		{name: "all packages in existing returns false", requested: []string{"make", "gcc"}},
		{name: "new package returns true", requested: []string{"make", "curl"}, want: true},
		{name: "empty requested returns false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			result := hasNewAptPackages(tc.requested, []string{"make", "gcc", "jq"})

			// Assert
			assert.Equal(t, tc.want, result)
		})
	}
}
