package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCleanCmd() *cobra.Command {
	cmd := &cobra.Command{}
	addNamespaceFlag(cmd)
	addAllFlag(cmd)
	return cmd
}

func Test_runClean(t *testing.T) {
	t.Run("cleans images and global resources when no args", func(t *testing.T) {
		// Arrange - confirms runClean wires clean.Resolve -> clean.Apply -> clean.GlobalResources
		// together when no tool arg is given; output formatting and per-target cleanup mechanics
		// are covered by internal/usecase/clean's own tests.
		t.Chdir(t.TempDir())
		logBuf := stubErrLog(t)
		var cleaned []string
		basesCleaned := false
		stubDocker(t, &fakeDocker{
			cleanImage: func(image string) error {
				cleaned = append(cleaned, image)
				return nil
			},
			cleanBaseImages: func() error {
				basesCleaned = true
				return nil
			},
		})

		// Act
		err := runClean(newTestCleanCmd(), []string{})

		// Assert
		require.NoError(t, err)
		assert.Contains(t, cleaned, "agentic-claude")
		assert.True(t, basesCleaned)
		assert.Contains(t, logBuf.String(), "agentic: removing 3 image(s): ")
	})

	t.Run("invalid project config fails fast with a clear error", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		rcPath := filepath.Join(dir, ".agenticrc.toml")
		require.NoError(t, os.WriteFile(rcPath, []byte("not valid toml [[["), 0o644))
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
		err := runClean(newTestCleanCmd(), []string{})

		// Assert
		require.Error(t, err)
	})

	t.Run("args present skips global resources", func(t *testing.T) {
		// Arrange
		basesCleaned := false
		stubDocker(t, &fakeDocker{
			cleanBaseImages: func() error {
				basesCleaned = true
				return nil
			},
		})

		// Act
		err := runClean(newTestCleanCmd(), []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.False(t, basesCleaned)
	})

	t.Run("all flag cleans across namespaces and base", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		var cleaned []string
		basesCleaned := false
		stubDocker(t, &fakeDocker{
			listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
				return []*docker.ImageInfo{
					{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
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
		cmd := newTestCleanCmd()
		require.NoError(t, cmd.Flags().Set("all", "true"))

		// Act
		err := runClean(cmd, []string{})

		// Assert
		require.NoError(t, err)
		// tool images across namespaces plus the global proxy image
		assert.ElementsMatch(t, []string{"agentic-claude", "work-claude", tools.ProxyImage, tools.DindImage}, cleaned)
		assert.True(t, basesCleaned)
	})

	t.Run("all flag with tool arg skips base", func(t *testing.T) {
		// Arrange
		basesCleaned := false
		stubDocker(t, &fakeDocker{
			listAllImages: func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
				return []*docker.ImageInfo{
					{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
				}, nil
			},
			cleanBaseImages: func() error {
				basesCleaned = true
				return nil
			},
		})
		cmd := newTestCleanCmd()
		require.NoError(t, cmd.Flags().Set("all", "true"))

		// Act
		err := runClean(cmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.False(t, basesCleaned)
	})
}
