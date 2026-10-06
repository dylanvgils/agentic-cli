package clean

import (
	"bytes"
	"fmt"
	"testing"

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
		targets, err := New(d).Resolve(Scope{All: true, FilterTool: "claude"})

		// Assert
		require.NoError(t, err)
		require.Len(t, targets, 1)
		assert.Equal(t, []docker.ImageFilter{docker.ToolFilter("claude")}, capturedFilters)
	})

	t.Run("scoped resolve dispatches to resolveScoped", func(t *testing.T) {
		// Act
		targets, err := New(&fakeDocker{}).Resolve(Scope{Names: []string{"claude"}, Namespace: "agentic"})

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
		_, err := New(d).resolveAll("claude")

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
		_, err := New(d).resolveAll("")

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
		targets, err := New(d).resolveAll("")

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
			err := New(d).Apply(targets)
			require.NoError(t, err)
		})

		// Assert
		assert.Equal(t, []string{"agentic-claude", "agentic-copilot"}, cleaned)
		assert.Contains(t, out, "=> agentic-claude")
		assert.Contains(t, out, "=> agentic-copilot")
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
		err := New(d).Apply(targets)

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
			removeNetwork: func() error {
				networkRemoved = true
				return nil
			},
		}

		// Act
		out := captureStdout(t, func() {
			err := New(d).GlobalResources("/agentic-home")
			require.NoError(t, err)
		})

		// Assert
		assert.True(t, basesCleaned)
		assert.Contains(t, cleaned, tools.ProxyImage)
		assert.Contains(t, cleaned, tools.DindImage)
		assert.Equal(t, []string{"dind", "proxy"}, swept, "sidecars must go before the proxy networks they sit on")
		assert.Equal(t, "/agentic-home", sweptHome)
		assert.True(t, networkRemoved)
		assert.Contains(t, out, "=> base")
		assert.Contains(t, out, "=> "+tools.ProxyImage)
		assert.Contains(t, out, "=> network")
		assert.Contains(t, logBuf.String(), "agentic: removing shared resources: base images, agentic-proxy, agentic-dind, sidecars, agentic-net")
	})

	t.Run("cleanBaseImages error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{cleanBaseImages: func() error { return fmt.Errorf("base cleanup failed") }}

		// Act
		err := New(d).GlobalResources("/agentic-home")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "base cleanup failed")
	})

	t.Run("cleanImage error for proxy propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{cleanImage: func(string) error { return fmt.Errorf("proxy cleanup failed") }}

		// Act
		err := New(d).GlobalResources("/agentic-home")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "proxy cleanup failed")
	})

	t.Run("sweepDindResources error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{sweepDindResources: func(string) error { return fmt.Errorf("dind sweep failed") }}

		// Act
		err := New(d).GlobalResources("/agentic-home")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dind sweep failed")
	})

	t.Run("sweepProxyResources error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{sweepProxyResources: func() error { return fmt.Errorf("sweep failed") }}

		// Act
		err := New(d).GlobalResources("/agentic-home")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "sweep failed")
	})

	t.Run("removeNetwork error propagates", func(t *testing.T) {
		// Arrange
		d := &fakeDocker{removeNetwork: func() error { return fmt.Errorf("network removal failed") }}

		// Act
		err := New(d).GlobalResources("/agentic-home")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "network removal failed")
	})
}
