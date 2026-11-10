package clean

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errFake is a docker failure the tests expect to see returned.
var errFake = errors.New("docker error")

func TestResolve(t *testing.T) {
	t.Run("all scope dispatches to resolveAll", func(t *testing.T) {
		// Arrange
		var capturedFilters []docker.ImageFilter
		d := &fakeDocker{
			listAllImages: func(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
				capturedFilters = filters
				return []*docker.ImageInfo{
					{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
				}, nil
			},
		}

		// Act
		targets, err := New(t.TempDir(), d).Resolve(Scope{All: true, Tool: "claude"})

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, []docker.ImageFilter{docker.ToolFilter("claude")}, capturedFilters)
	})

	t.Run("single tool resolves to its image in the namespace", func(t *testing.T) {
		// Act
		targets, err := New(t.TempDir(), &fakeDocker{}).Resolve(Scope{Tool: "claude", Namespace: "agentic"})

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, "agentic-claude", targets[0].Label)
		assert.Equal(t, "agentic-claude", targets[0].Image)
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		_, err := New(t.TempDir(), &fakeDocker{}).Resolve(Scope{Tool: "nonexistent", Namespace: "agentic"})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown tool")
	})
}

func Test_resolveAll(t *testing.T) {
	t.Run("skips the proxy image since GlobalResources handles it separately", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
				return []*docker.ImageInfo{
					{Image: "agentic-proxy", Namespace: "agentic", Tool: "proxy"},
					{Image: "work-claude", Namespace: "work", Tool: "claude"},
				}, nil
			},
		}

		// Act
		targets, err := New(t.TempDir(), d).resolveAll("")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []Target{{Label: "work/claude", Image: "work-claude"}}, targets)
	})

	t.Run("list error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
				return nil, fmt.Errorf("docker error")
			},
		}

		// Act
		_, err := New(t.TempDir(), d).resolveAll("")

		// Assert
		assert.ErrorContains(t, err, "docker error")
	})
}

func TestApply(t *testing.T) {
	targets := []Target{
		{Label: "agentic-claude", Image: "agentic-claude"},
		{Label: "agentic-copilot", Image: "agentic-copilot"},
	}

	t.Run("cleans each target after its dockerd image", func(t *testing.T) {
		// Arrange
		var cleaned []string
		d := &fakeDocker{cleanImage: recordCleans(&cleaned, "")}

		// Act
		var err error
		out := iostreamstest.CaptureErr(t, func() { err = New(t.TempDir(), d).Apply(targets) })

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"agentic-claude-dockerd", "agentic-claude", "agentic-copilot-dockerd", "agentic-copilot"}, cleaned)
		assert.Equal(t, "=> agentic-claude\n=> agentic-copilot\n", out)
	})

	t.Run("cleans each target from the other runtimes", func(t *testing.T) {
		// Arrange
		var cleaned []string
		m := &fakeRuntime{cleanImage: recordCleans(&cleaned, "")}

		// Act
		iostreamstest.CaptureErr(t, func() {
			err := New(t.TempDir(), &fakeDocker{}, m).Apply(targets)
			require.NoError(t, err)
		})

		// Assert
		assert.Equal(t, []string{"agentic-claude-dockerd", "agentic-claude", "agentic-copilot-dockerd", "agentic-copilot"}, cleaned)
	})

	t.Run("warns and goes on when another runtime fails", func(t *testing.T) {
		// Arrange
		errLog := iostreamstest.StubErr(t)
		var cleaned []string
		d := &fakeDocker{cleanImage: recordCleans(&cleaned, "")}
		m := &fakeRuntime{
			cleanImage: func(image string) error {
				return fmt.Errorf("fail on %s", image)
			},
		}

		// Act
		err := New(t.TempDir(), d, m).Apply(targets)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"agentic-claude-dockerd", "agentic-claude", "agentic-copilot-dockerd", "agentic-copilot"}, cleaned)
		assert.Contains(t, errLog.String(), "kept agentic-claude: fail on agentic-claude")
	})

	t.Run("removes stored custom installs no image uses anymore", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		helm := []config.RCCustomInstall{{Name: "helm", Run: []string{"echo helm"}}}
		lint := []config.RCCustomInstall{{Name: "golangci-lint", Run: []string{"echo lint"}}}
		require.NoError(t, config.SaveCustomInstalls(home, helm))
		require.NoError(t, config.SaveCustomInstalls(home, lint))
		images := map[string]*docker.ImageInfo{
			"agentic-claude":  {Image: "agentic-claude", CustomInstallsHash: config.CustomInstallsHash(helm)},
			"agentic-copilot": {Image: "agentic-copilot", CustomInstallsHash: config.CustomInstallsHash(lint)},
			"work-claude":     {Image: "work-claude", CustomInstallsHash: config.CustomInstallsHash(lint)},
		}
		d := &fakeDocker{
			listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
				var remaining []*docker.ImageInfo
				for _, info := range images {
					remaining = append(remaining, info)
				}
				return remaining, nil
			},
			cleanImage: func(image string) error {
				delete(images, image)
				return nil
			},
		}

		// Act
		iostreamstest.CaptureErr(t, func() {
			err := New(home, d).Apply(targets)
			require.NoError(t, err)
		})

		// Assert - lint is still used by work-claude
		_, helmStored := config.LoadCustomInstalls(home, config.CustomInstallsHash(helm))
		_, lintStored := config.LoadCustomInstalls(home, config.CustomInstallsHash(lint))
		assert.False(t, helmStored)
		assert.True(t, lintStored)
	})

	t.Run("listing failure leaves stored custom installs alone", func(t *testing.T) {
		// Arrange
		home := t.TempDir()
		helm := []config.RCCustomInstall{{Name: "helm", Run: []string{"echo helm"}}}
		require.NoError(t, config.SaveCustomInstalls(home, helm))
		d := &fakeDocker{
			listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
				return nil, fmt.Errorf("docker error")
			},
		}

		// Act
		iostreamstest.CaptureErr(t, func() {
			err := New(home, d).Apply(targets)
			require.NoError(t, err)
		})

		// Assert
		_, stored := config.LoadCustomInstalls(home, config.CustomInstallsHash(helm))
		assert.True(t, stored)
	})

	t.Run("keeps an image an active run uses", func(t *testing.T) {
		// Arrange
		var cleaned []string
		d := &fakeDocker{
			listRunning: func() ([]*docker.ContainerInfo, error) {
				return []*docker.ContainerInfo{{Name: "agentic-claude-abc", Image: "agentic-claude"}}, nil
			},
			cleanImage: recordCleans(&cleaned, ""),
		}
		m := &fakeRuntime{cleanImage: recordCleans(&cleaned, "msb ")}

		// Act
		iostreamstest.CaptureErr(t, func() {
			err := New(t.TempDir(), d, m).Apply(targets)
			require.NoError(t, err)
		})

		// Assert
		assert.Equal(t, []string{"agentic-copilot-dockerd", "msb agentic-copilot-dockerd", "agentic-copilot", "msb agentic-copilot"}, cleaned)
	})

	t.Run("running containers list error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			listRunning: func() ([]*docker.ContainerInfo, error) { return nil, fmt.Errorf("docker error") },
		}

		// Act
		err := New(t.TempDir(), d).Apply(targets)

		// Assert
		assert.ErrorContains(t, err, "docker error")
	})

	t.Run("stops on first error", func(t *testing.T) {
		// Arrange
		var cleaned []string
		d := &fakeDocker{
			cleanImage: func(image string) error {
				cleaned = append(cleaned, image)
				return fmt.Errorf("fail on %s", image)
			},
		}

		// Act
		err := New(t.TempDir(), d).Apply(targets)

		// Assert
		require.Error(t, err)
		assert.Len(t, cleaned, 1)
	})
}

func TestGlobalResources(t *testing.T) {
	t.Run("cleans base, proxy image, sweeps, and removes network", func(t *testing.T) {
		// Arrange
		logBuf := iostreamstest.StubErr(t)
		var cleaned []string
		basesCleaned := false
		var swept []string
		var sweptHome string
		networkRemoved := false
		d := &fakeDocker{
			cleanImage: func(image string) error {
				cleaned = append(cleaned, image)
				return nil
			},
			cleanBaseImages: func() error {
				basesCleaned = true
				return nil
			},
			sweepRuns: func(toolHome string) (docker.SweptRuns, error) {
				sweptHome = toolHome
				swept = append(swept, "runs")
				return docker.SweptRuns{}, nil
			},
			listRunning: func() ([]*docker.ContainerInfo, error) {
				swept = append(swept, "in use")
				return nil, nil
			},
			removeNetwork: func() error {
				networkRemoved = true
				return nil
			},
		}

		// Act
		err := New("/agentic-home", d).GlobalResources()

		// Assert
		require.NoError(t, err)
		out := logBuf.String()
		assert.True(t, basesCleaned)
		assert.Contains(t, cleaned, tools.ProxyImage)
		assert.Contains(t, cleaned, tools.DindImage)
		assert.Equal(t, []string{"runs", "in use"}, swept, "interrupted runs must go before the in-use check")
		assert.Equal(t, "/agentic-home", sweptHome)
		assert.True(t, networkRemoved)
		assert.Contains(t, out, "=> base")
		assert.Contains(t, out, "=> "+tools.ProxyImage)
		assert.Contains(t, out, "=> network")
		assert.Contains(t, out, "agentic: removing shared resources")
	})

	for _, tc := range []struct {
		name string
		d    *fakeDocker
	}{
		{name: "base images", d: &fakeDocker{cleanBaseImages: func() error { return errFake }}},
		{name: "sidecar image", d: &fakeDocker{cleanImage: func(string) error { return errFake }}},
		{
			name: "run sweep",
			d: &fakeDocker{sweepRuns: func(string) (docker.SweptRuns, error) {
				return docker.SweptRuns{}, errFake
			}},
		},
		{name: "network removal", d: &fakeDocker{removeNetwork: func() error { return errFake }}},
	} {
		t.Run(tc.name+" error propagates", func(t *testing.T) {
			// Act
			var err error
			iostreamstest.CaptureErr(t, func() { err = New("/agentic-home", tc.d).GlobalResources() })

			// Assert
			assert.ErrorIs(t, err, errFake)
		})
	}
}

func TestPrune(t *testing.T) {
	t.Run("sweeps runs, then prunes images and the confirmed cache", func(t *testing.T) {
		// Arrange
		var ran []string
		var sweptHome string
		d := &fakeDocker{
			sweepRuns: func(toolHome string) (docker.SweptRuns, error) {
				sweptHome = toolHome
				ran = append(ran, "runs")
				return docker.SweptRuns{
					Sidecars: []string{"agentic-proxy-abc"},
					Networks: []string{"agentic-run-abc"},
				}, nil
			},
			pruneImages: func() (string, error) {
				ran = append(ran, "images")
				return "", nil
			},
			pruneBuildCache: func() (string, error) {
				ran = append(ran, "cache")
				return "", nil
			},
		}

		// Act
		out := iostreamstest.CaptureErr(t, func() {
			err := New("/agentic-home", d).Prune(func() bool { return true })
			require.NoError(t, err)
		})

		// Assert
		assert.Equal(t, []string{"runs", "images", "cache"}, ran)
		assert.Equal(t, "/agentic-home", sweptHome)
		assert.Contains(t, out, "=> leftover sidecars\n   agentic-proxy-abc\n=> leftover run networks\n   agentic-run-abc\n")
		assert.Contains(t, out, "=> build cache\n")
	})

	t.Run("keeps the build cache when not confirmed", func(t *testing.T) {
		// Arrange
		cachePruned := false
		d := &fakeDocker{pruneBuildCache: func() (string, error) {
			cachePruned = true
			return "", nil
		}}

		// Act
		out := iostreamstest.CaptureErr(t, func() {
			err := New("/agentic-home", d).Prune(func() bool { return false })
			require.NoError(t, err)
		})

		// Assert
		assert.False(t, cachePruned)
		assert.Contains(t, out, "=> build cache (kept)")
	})

	t.Run("a failed sweep stops before pruning", func(t *testing.T) {
		// Arrange
		imagesPruned := false
		d := &fakeDocker{
			sweepRuns: func(string) (docker.SweptRuns, error) { return docker.SweptRuns{}, fmt.Errorf("sweep failed") },
			pruneImages: func() (string, error) {
				imagesPruned = true
				return "", nil
			},
		}

		// Act
		var err error
		iostreamstest.CaptureErr(t, func() {
			err = New("/agentic-home", d).Prune(func() bool { return true })
		})

		// Assert
		require.ErrorContains(t, err, "sweep failed")
		assert.False(t, imagesPruned)
	})
}

func Test_cleanSidecarImages(t *testing.T) {
	t.Run("keeps a sidecar image an active run uses", func(t *testing.T) {
		// Arrange
		var cleaned []string
		d := &fakeDocker{
			listRunning: func() ([]*docker.ContainerInfo, error) {
				return []*docker.ContainerInfo{{Name: "agentic-proxy-abc", Image: tools.ProxyImage}}, nil
			},
			cleanImage: recordCleans(&cleaned, ""),
		}

		// Act
		var err error
		iostreamstest.CaptureErr(t, func() { err = New("/agentic-home", d).cleanSidecarImages() })

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{tools.DindImage}, cleaned)
	})

	t.Run("running containers list error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			listRunning: func() ([]*docker.ContainerInfo, error) { return nil, fmt.Errorf("docker error") },
		}

		// Act
		err := New("/agentic-home", d).cleanSidecarImages()

		// Assert
		assert.ErrorContains(t, err, "docker error")
	})
}

func Test_stepImage(t *testing.T) {
	for _, tc := range []struct {
		name  string
		inUse bool
		want  string
	}{
		{name: "image in use is marked as kept", inUse: true, want: "=> agentic-claude (in use, kept)\n"},
		{name: "unused image is a plain step", want: "=> agentic-claude\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			var kept bool
			out := iostreamstest.CaptureErr(t, func() { kept = stepImage("agentic-claude", tc.inUse) })

			// Assert
			assert.Equal(t, tc.inUse, kept)
			assert.Equal(t, tc.want, out)
		})
	}
}

func Test_stepRemoved(t *testing.T) {
	for _, tc := range []struct {
		name    string
		removed []string
		want    string
	}{
		{
			name:    "lists the removed names under the label",
			removed: []string{"agentic-docker-abc", "agentic-proxy-abc"},
			want:    "=> leftover sidecars\n   agentic-docker-abc\n   agentic-proxy-abc\n",
		},
		{
			name: "reports none when nothing was removed",
			want: "=> leftover sidecars\n   none\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			out := iostreamstest.CaptureErr(t, func() { stepRemoved("leftover sidecars", tc.removed) })

			// Assert
			assert.Equal(t, tc.want, out)
		})
	}
}

func Test_stepReclaimed(t *testing.T) {
	for _, tc := range []struct {
		name string
		size string
		err  error
		want string
	}{
		{name: "reports the reclaimed space under the label", size: "1.5GB", want: "=> build cache\n   reclaimed 1.5GB\n"},
		{name: "an unknown size prints only the label", want: "=> build cache\n"},
		{name: "returns the error", err: errFake, want: "=> build cache\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			var err error
			out := iostreamstest.CaptureErr(t, func() {
				err = stepReclaimed("build cache", func() (string, error) { return tc.size, tc.err })
			})

			// Assert
			assert.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.want, out)
		})
	}
}
