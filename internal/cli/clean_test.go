package cli

import (
	"fmt"
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
		// Arrange
		t.Chdir(t.TempDir())
		var cleaned []string
		stubCleanCleanImage(t, func(image string) error {
			cleaned = append(cleaned, image)
			return nil
		})
		basesCleaned := false
		stubCleanCleanBaseImages(t, func() error {
			basesCleaned = true
			return nil
		})
		stubCleanSweepProxyResources(t, func() error { return nil })
		stubCleanRemoveNetwork(t, func() error { return nil })

		// Act
		out := captureStdout(t, func() {
			err := runClean(newTestCleanCmd(), []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "=> agentic-claude")
		assert.Contains(t, out, "=> base")
		assert.True(t, basesCleaned)
	})

	t.Run("propagates error from cleanTargets", func(t *testing.T) {
		// Arrange
		stubCleanCleanImage(t, func(image string) error { return fmt.Errorf("fail on %s", image) })

		// Act
		err := runClean(newTestCleanCmd(), []string{})

		// Assert
		require.Error(t, err)
	})

	t.Run("args present skips global resources", func(t *testing.T) {
		// Arrange
		stubCleanCleanImage(t, func(string) error { return nil })
		basesCleaned := false
		stubCleanCleanBaseImages(t, func() error {
			basesCleaned = true
			return nil
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
		stubCleanListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
				{Image: "work-claude", Namespace: "work", Tool: "claude"},
			}, nil
		})
		var cleaned []string
		stubCleanCleanImage(t, func(image string) error {
			cleaned = append(cleaned, image)
			return nil
		})
		basesCleaned := false
		stubCleanCleanBaseImages(t, func() error {
			basesCleaned = true
			return nil
		})
		stubCleanSweepProxyResources(t, func() error { return nil })
		cmd := newTestCleanCmd()
		require.NoError(t, cmd.Flags().Set("all", "true"))

		// Act
		err := runClean(cmd, []string{})

		// Assert
		require.NoError(t, err)
		// tool images across namespaces plus the global proxy image
		assert.ElementsMatch(t, []string{"agentic-claude", "work-claude", tools.ProxyImage}, cleaned)
		assert.True(t, basesCleaned)
	})

	t.Run("all flag with tool arg skips base", func(t *testing.T) {
		// Arrange
		stubCleanListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
			}, nil
		})
		stubCleanCleanImage(t, func(_ string) error { return nil })
		basesCleaned := false
		stubCleanCleanBaseImages(t, func() error {
			basesCleaned = true
			return nil
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
