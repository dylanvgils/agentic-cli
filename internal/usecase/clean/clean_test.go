package clean

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureStdout swaps logging.Log for the duration of fn and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	var buf bytes.Buffer
	orig := logging.Log
	logging.Log = logging.New(&buf)
	t.Cleanup(func() { logging.Log = orig })

	fn()

	return buf.String()
}

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
		targets, err := New(t.TempDir(), d).Resolve(Scope{All: true, FilterTool: "claude"})

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, []docker.ImageFilter{docker.ToolFilter("claude")}, capturedFilters)
	})

	t.Run("scoped resolve dispatches to resolveScoped", func(t *testing.T) {
		// Act
		targets, err := New(t.TempDir(), &fakeDocker{}).Resolve(Scope{Names: []string{"claude"}, Namespace: "agentic"})

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, "agentic-claude", targets[0].Image)
	})
}

func Test_resolveScoped(t *testing.T) {
	t.Run("known tool resolves to a target", func(t *testing.T) {
		// Act
		targets, err := resolveScoped([]string{"claude"}, "agentic")

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, "agentic-claude", targets[0].Label)
		assert.Equal(t, "agentic-claude", targets[0].Image)
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		_, err := resolveScoped([]string{"nonexistent"}, "agentic")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown tool")
	})
}

func Test_resolveAll(t *testing.T) {
	t.Run("tool arg applies filter", func(t *testing.T) {
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
		_, err := New(t.TempDir(), d).resolveAll("claude")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []docker.ImageFilter{docker.ToolFilter("claude")}, capturedFilters)
	})

	t.Run("listAllImages error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
				return nil, fmt.Errorf("docker error")
			},
		}

		// Act
		_, err := New(t.TempDir(), d).resolveAll("")

		// Assert
		require.Error(t, err)
	})

	t.Run("skips the proxy image since GlobalResources handles it separately", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{
			listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
				return []*docker.ImageInfo{
					{Image: "agentic-proxy", Namespace: "agentic", Tool: "proxy"},
					{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
				}, nil
			},
		}

		// Act
		targets, err := New(t.TempDir(), d).resolveAll("")

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, "agentic-claude", targets[0].Image)
	})
}

func TestApply(t *testing.T) {
	targets := []Target{
		{Label: "agentic-claude", Image: "agentic-claude"},
		{Label: "agentic-copilot", Image: "agentic-copilot"},
	}

	t.Run("cleans each target", func(t *testing.T) {
		// Arrange
		var cleaned []string
		d := &fakeDocker{
			cleanImage: func(image string) error {
				cleaned = append(cleaned, image)
				return nil
			},
		}

		// Act
		out := captureStdout(t, func() {
			err := New(t.TempDir(), d).Apply(targets)
			require.NoError(t, err)
		})

		// Assert
		assert.Equal(t, []string{"agentic-claude", "agentic-copilot"}, cleaned)
		assert.Contains(t, out, "=> agentic-claude")
		assert.Contains(t, out, "=> agentic-copilot")
	})

	t.Run("cleans each target from the other runtimes", func(t *testing.T) {
		// Arrange
		var cleaned []string
		m := &fakeRuntime{
			cleanImage: func(image string) error {
				cleaned = append(cleaned, image)
				return nil
			},
		}

		// Act
		captureStdout(t, func() {
			err := New(t.TempDir(), &fakeDocker{}, m).Apply(targets)
			require.NoError(t, err)
		})

		// Assert
		assert.Equal(t, []string{"agentic-claude", "agentic-copilot"}, cleaned)
	})

	t.Run("warns and goes on when another runtime fails", func(t *testing.T) {
		// Arrange
		errLog := stubErrLog(t)
		var cleaned []string
		d := &fakeDocker{
			cleanImage: func(image string) error {
				cleaned = append(cleaned, image)
				return nil
			},
		}
		m := &fakeRuntime{
			cleanImage: func(image string) error {
				return fmt.Errorf("fail on %s", image)
			},
		}

		// Act
		captureStdout(t, func() {
			err := New(t.TempDir(), d, m).Apply(targets)
			require.NoError(t, err)
		})

		// Assert
		assert.Equal(t, []string{"agentic-claude", "agentic-copilot"}, cleaned)
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
		captureStdout(t, func() {
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
		captureStdout(t, func() {
			err := New(home, d).Apply(targets)
			require.NoError(t, err)
		})

		// Assert
		_, stored := config.LoadCustomInstalls(home, config.CustomInstallsHash(helm))
		assert.True(t, stored)
	})

	t.Run("keeps an image an active run uses", func(t *testing.T) {
		// Arrange
		errLog := stubErrLog(t)
		var cleaned []string
		d := &fakeDocker{
			listRunning: func() ([]*docker.ContainerInfo, error) {
				return []*docker.ContainerInfo{{Name: "agentic-claude-abc", Image: "agentic-claude"}}, nil
			},
			cleanImage: func(image string) error {
				cleaned = append(cleaned, image)
				return nil
			},
		}
		m := &fakeRuntime{
			cleanImage: func(image string) error {
				cleaned = append(cleaned, "msb "+image)
				return nil
			},
		}

		// Act
		captureStdout(t, func() {
			err := New(t.TempDir(), d, m).Apply(targets)
			require.NoError(t, err)
		})

		// Assert
		assert.Equal(t, []string{"agentic-copilot", "msb agentic-copilot"}, cleaned)
		assert.Contains(t, errLog.String(), "kept agentic-claude: in use by agentic-claude-abc")
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
		logBuf := stubErrLog(t)
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
			sweepDindResources: func(toolHome string) error {
				sweptHome = toolHome
				swept = append(swept, "dind")
				return nil
			},
			sweepProxyResources: func() error {
				swept = append(swept, "proxy")
				return nil
			},
			sweepRunNetworks: func() error {
				swept = append(swept, "networks")
				return nil
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
		out := captureStdout(t, func() {
			err := New("/agentic-home", d).GlobalResources()
			require.NoError(t, err)
		})

		// Assert
		assert.True(t, basesCleaned)
		assert.Contains(t, cleaned, tools.ProxyImage)
		assert.Contains(t, cleaned, tools.DindImage)
		assert.Equal(t, []string{"dind", "proxy", "in use", "networks"}, swept, "orphaned sidecars must go before the in-use check and the run networks they sit on")
		assert.Equal(t, "/agentic-home", sweptHome)
		assert.True(t, networkRemoved)
		assert.Contains(t, out, "=> base")
		assert.Contains(t, out, "=> "+tools.ProxyImage)
		assert.Contains(t, out, "=> network")
		assert.Contains(t, logBuf.String(), "agentic: removing shared resources: base images, agentic-proxy, agentic-dind, sidecars, run networks, agentic-net")
	})

	t.Run("cleanBaseImages error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{cleanBaseImages: func() error { return fmt.Errorf("base cleanup failed") }}

		// Act
		err := New("/agentic-home", d).GlobalResources()

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "base cleanup failed")
	})

	t.Run("cleanImage error for proxy propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{cleanImage: func(string) error { return fmt.Errorf("proxy cleanup failed") }}

		// Act
		err := New("/agentic-home", d).GlobalResources()

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "proxy cleanup failed")
	})

	t.Run("sweepDindResources error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{sweepDindResources: func(string) error { return fmt.Errorf("dind sweep failed") }}

		// Act
		err := New("/agentic-home", d).GlobalResources()

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dind sweep failed")
	})

	t.Run("sweepProxyResources error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{sweepProxyResources: func() error { return fmt.Errorf("sweep failed") }}

		// Act
		err := New("/agentic-home", d).GlobalResources()

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "sweep failed")
	})

	t.Run("sweepRunNetworks error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{sweepRunNetworks: func() error { return fmt.Errorf("network sweep failed") }}

		// Act
		err := New("/agentic-home", d).GlobalResources()

		// Assert
		assert.ErrorContains(t, err, "network sweep failed")
	})

	t.Run("removeNetwork error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{removeNetwork: func() error { return fmt.Errorf("network removal failed") }}

		// Act
		err := New("/agentic-home", d).GlobalResources()

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "network removal failed")
	})
}

func Test_cleanSidecarImages(t *testing.T) {
	t.Run("keeps a sidecar image an active run uses", func(t *testing.T) {
		// Arrange
		errLog := stubErrLog(t)
		var cleaned []string
		d := &fakeDocker{
			listRunning: func() ([]*docker.ContainerInfo, error) {
				return []*docker.ContainerInfo{{Name: "agentic-proxy-abc", Image: tools.ProxyImage}}, nil
			},
			cleanImage: func(image string) error {
				cleaned = append(cleaned, image)
				return nil
			},
		}

		// Act
		var err error
		captureStdout(t, func() { err = New("/agentic-home", d).cleanSidecarImages() })

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{tools.DindImage}, cleaned)
		assert.Contains(t, errLog.String(), "kept agentic-proxy: in use by agentic-proxy-abc")
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

func Test_keptInUse(t *testing.T) {
	inUse := map[string]string{"agentic-claude": "agentic-claude-abc"}

	t.Run("image in use is kept with a warning", func(t *testing.T) {
		// Arrange
		errLog := stubErrLog(t)

		// Act
		kept := keptInUse("agentic-claude", inUse)

		// Assert
		assert.True(t, kept)
		assert.Contains(t, errLog.String(), "kept agentic-claude: in use by agentic-claude-abc")
	})

	t.Run("unused image is not kept", func(t *testing.T) {
		// Arrange
		errLog := stubErrLog(t)

		// Act
		kept := keptInUse("agentic-copilot", inUse)

		// Assert
		assert.False(t, kept)
		assert.Empty(t, errLog.String())
	})
}
