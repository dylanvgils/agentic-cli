package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runClean(t *testing.T) {
	// Keep the repo's own .agenticrc.toml out of the test
	t.Chdir(t.TempDir())
	// Keep a host msb's images out of the test
	stubMicrosandbox(t, &fakeMicrosandbox{})

	t.Run("cleans images and global resources with all and no tool", func(t *testing.T) {
		// Arrange - confirms runClean wires clean.Resolve -> clean.Apply -> clean.GlobalResources
		// together for a bare --all; output formatting and per-target cleanup mechanics
		// are covered by internal/usecase/clean's own tests.
		t.Chdir(t.TempDir())
		logBuf := iostreamstest.StubErr(t)
		var cleaned []string
		basesCleaned := false
		stubDocker(t, &fakeDocker{
			listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
				return []*docker.ImageInfo{
					{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
					{Image: "agentic-copilot", Namespace: "agentic", Tool: "copilot"},
					{Image: "work-claude", Namespace: "work", Tool: "claude"},
				}, nil
			},
			cleanImage: func(image string) error {
				cleaned = append(cleaned, image)
				return nil
			},
			cleanBaseImages: func() error {
				basesCleaned = true
				return nil
			},
		})
		var msbCleaned []string
		stubMicrosandbox(t, &fakeMicrosandbox{
			cleanImage: func(image string) error {
				msbCleaned = append(msbCleaned, image)
				return nil
			},
		})
		cmd := newTestCleanCmd()
		require.NoError(t, cmd.Flags().Set(allFlagName, "true"))

		// Act
		err := runClean(cmd, []string{})

		// Assert
		require.NoError(t, err)
		// tool images and their dockerd images across namespaces, plus the global sidecar images
		assert.ElementsMatch(t, []string{
			"agentic-claude-dockerd",
			"agentic-claude",
			"agentic-copilot-dockerd",
			"agentic-copilot",
			"work-claude-dockerd",
			"work-claude",
			tools.ProxyImage,
			tools.DindImage,
		}, cleaned)
		assert.Contains(t, msbCleaned, "agentic-claude")
		assert.True(t, basesCleaned)
		assert.Contains(t, logBuf.String(), "agentic: removing 3 image(s): ")
		assert.Contains(t, logBuf.String(), "\n\nagentic: removing shared resources", "a blank line sets the shared resources apart")
	})

	t.Run("skips microsandbox when msb isn't installed", func(t *testing.T) {
		// Arrange
		stubDocker(t, &fakeDocker{})
		msbCleaned := false
		stubMicrosandbox(t, &fakeMicrosandbox{
			available: func() error { return errors.New("msb not found") },
			cleanImage: func(string) error {
				msbCleaned = true
				return nil
			},
		})

		// Act
		err := runClean(newTestCleanCmd(), []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.False(t, msbCleaned)
	})

	t.Run("invalid project config fails fast with a clear error", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		writeRC(t, dir, "not valid toml [[[")
		rcPath := filepath.Join(dir, ".agenticrc.toml")
		t.Chdir(dir)

		// Act
		err := runClean(newTestCleanCmd(), []string{"claude"})

		// Assert
		assert.ErrorContains(t, err, rcPath)
	})

	t.Run("propagates error from cleanTargets", func(t *testing.T) {
		// Arrange
		stubDocker(t, &fakeDocker{
			cleanImage: func(image string) error { return fmt.Errorf("fail on %s", image) },
		})

		// Act
		err := runClean(newTestCleanCmd(), []string{"claude"})

		// Assert
		assert.ErrorContains(t, err, "fail on agentic-claude")
	})

	t.Run("a tool skips global resources, even with all", func(t *testing.T) {
		// Arrange
		basesCleaned := false
		stubDocker(t, &fakeDocker{
			cleanBaseImages: func() error {
				basesCleaned = true
				return nil
			},
		})
		cmd := newTestCleanCmd()
		require.NoError(t, cmd.Flags().Set(allFlagName, "true"))

		// Act
		err := runClean(cmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.False(t, basesCleaned)
	})
}
