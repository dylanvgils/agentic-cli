package cmd

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
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

func Test_resolveScopedCleanTargets(t *testing.T) {
	// Act
	targets, err := resolveScopedCleanTargets([]string{"claude"}, "agentic")

	// Assert
	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.Equal(t, "agentic-claude", targets[0].label)
	assert.Equal(t, "agentic-claude", targets[0].image)
}

func Test_resolveAllCleanTargets(t *testing.T) {
	t.Run("tool arg applies filter", func(t *testing.T) {
		// Arrange
		var capturedFilters []docker.ImageFilter
		stubListAllImages(t, func(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			capturedFilters = filters
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
			}, nil
		})

		// Act
		_, err := resolveAllCleanTargets([]string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []docker.ImageFilter{docker.ToolFilter("claude")}, capturedFilters)
	})

	t.Run("listAllImages error propagates", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, fmt.Errorf("docker error")
		})

		// Act
		_, err := resolveAllCleanTargets([]string{})

		// Assert
		require.Error(t, err)
	})
}

func Test_runClean(t *testing.T) {
	t.Run("cleans images and base when no args", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		var cleaned []string
		stubCleanImage(t, func(image string) error {
			cleaned = append(cleaned, image)
			return nil
		})
		basesCleaned := false
		stubCleanBaseImages(t, func() error {
			basesCleaned = true
			return nil
		})

		// Act
		out := captureStdout(t, func() {
			err := runClean(newTestCleanCmd(), []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "=> agentic-claude")
		assert.Contains(t, out, "=> base")
		assert.True(t, basesCleaned)
		assert.Len(t, cleaned, 3)
	})

	t.Run("stops on first cleanImage error", func(t *testing.T) {
		// Arrange
		var cleaned []string
		stubCleanImage(t, func(image string) error {
			cleaned = append(cleaned, image)
			return fmt.Errorf("fail on %s", image)
		})

		// Act
		err := runClean(newTestCleanCmd(), []string{})

		// Assert
		require.Error(t, err)
		assert.Len(t, cleaned, 1)
	})

	t.Run("cleanBaseImages error propagates", func(t *testing.T) {
		// Arrange
		stubCleanImage(t, func(_ string) error { return nil })
		stubCleanBaseImages(t, func() error { return fmt.Errorf("base cleanup failed") })

		// Act
		err := runClean(newTestCleanCmd(), []string{})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "base cleanup failed")
	})

	t.Run("all flag cleans across namespaces and base", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
				{Image: "work-claude", Namespace: "work", Tool: "claude"},
			}, nil
		})
		var cleaned []string
		stubCleanImage(t, func(image string) error {
			cleaned = append(cleaned, image)
			return nil
		})
		basesCleaned := false
		stubCleanBaseImages(t, func() error {
			basesCleaned = true
			return nil
		})
		cmd := newTestCleanCmd()
		require.NoError(t, cmd.Flags().Set("all", "true"))

		// Act
		err := runClean(cmd, []string{})

		// Assert
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"agentic-claude", "work-claude"}, cleaned)
		assert.True(t, basesCleaned)
	})

	t.Run("all flag with tool arg skips base", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
			}, nil
		})
		stubCleanImage(t, func(_ string) error { return nil })
		basesCleaned := false
		stubCleanBaseImages(t, func() error {
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
