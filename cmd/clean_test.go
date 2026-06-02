package cmd

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanScoped(t *testing.T) {
	t.Run("all tools when no args", func(t *testing.T) {
		// Arrange
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
			err := cleanScoped([]string{}, "agentic")
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "=> claude")
		assert.Contains(t, out, "=> copilot")
		assert.Contains(t, out, "=> opencode")
		assert.Contains(t, out, "=> base")
		assert.True(t, basesCleaned)
		assert.Len(t, cleaned, 3)
	})

	t.Run("single tool when arg given", func(t *testing.T) {
		// Arrange
		var cleaned []string
		stubCleanImage(t, func(image string) error {
			cleaned = append(cleaned, image)
			return nil
		})

		// Act
		out := captureStdout(t, func() {
			err := cleanScoped([]string{"claude"}, "agentic")
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "=> claude")
		assert.NotContains(t, out, "=> copilot")
		assert.NotContains(t, out, "=> opencode")
		assert.NotContains(t, out, "=> base")
		assert.Equal(t, []string{"agentic-claude"}, cleaned)
	})

	t.Run("stops on first tool error", func(t *testing.T) {
		// Arrange
		var cleaned []string
		stubCleanImage(t, func(image string) error {
			cleaned = append(cleaned, image)
			return fmt.Errorf("fail on %s", image)
		})

		// Act
		err := cleanScoped([]string{}, "agentic")

		// Assert
		require.Error(t, err)
		assert.Len(t, cleaned, 1)
	})

	t.Run("clean base images error propagates", func(t *testing.T) {
		// Arrange
		stubCleanImage(t, func(_ string) error { return nil })
		stubCleanBaseImages(t, func() error {
			return fmt.Errorf("base cleanup failed")
		})

		// Act
		err := cleanScoped([]string{}, "agentic")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "base cleanup failed")
	})
}

func TestCleanAll(t *testing.T) {
	t.Run("no args removes all images and base", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Prefix: "agentic", Tool: "claude"},
				{Image: "work-claude", Prefix: "work", Tool: "claude"},
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

		// Act
		err := cleanAll([]string{})

		// Assert
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"agentic-claude", "work-claude"}, cleaned)
		assert.True(t, basesCleaned)
	})

	t.Run("tool arg filters images and skips base", func(t *testing.T) {
		// Arrange
		var capturedFilters []docker.ImageFilter
		stubListAllImages(t, func(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			capturedFilters = filters
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Prefix: "agentic", Tool: "claude"},
				{Image: "work-claude", Prefix: "work", Tool: "claude"},
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

		// Act
		err := cleanAll([]string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []docker.ImageFilter{docker.ToolFilter("claude")}, capturedFilters)
		assert.ElementsMatch(t, []string{"agentic-claude", "work-claude"}, cleaned)
		assert.False(t, basesCleaned)
	})
}
