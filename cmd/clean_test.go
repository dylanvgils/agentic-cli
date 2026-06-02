package cmd

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunClean(t *testing.T) {
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
			err := runClean(cleanCmd, []string{})
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

		basesCleaned := false
		stubCleanBaseImages(t, func() error {
			basesCleaned = true
			return nil
		})

		// Act
		out := captureStdout(t, func() {
			err := runClean(cleanCmd, []string{"claude"})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "=> claude")
		assert.NotContains(t, out, "=> copilot")
		assert.NotContains(t, out, "=> opencode")
		assert.NotContains(t, out, "=> base")
		assert.False(t, basesCleaned)
		assert.Equal(t, []string{"agentic-claude"}, cleaned)
	})

	t.Run("clean image error propagates", func(t *testing.T) {
		// Arrange
		stubCleanImage(t, func(_ string) error {
			return fmt.Errorf("docker daemon not running")
		})
		stubCleanBaseImages(t, func() error { return nil })

		// Act
		err := runClean(cleanCmd, []string{"claude"})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})

	t.Run("clean base images error propagates", func(t *testing.T) {
		// Arrange
		stubCleanImage(t, func(_ string) error { return nil })
		stubCleanBaseImages(t, func() error {
			return fmt.Errorf("base cleanup failed")
		})

		// Act
		err := runClean(cleanCmd, []string{})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "base cleanup failed")
	})

	t.Run("stops on first tool error", func(t *testing.T) {
		// Arrange
		var cleaned []string
		stubCleanImage(t, func(image string) error {
			cleaned = append(cleaned, image)
			return fmt.Errorf("fail on %s", image)
		})
		stubCleanBaseImages(t, func() error { return nil })

		// Act
		err := runClean(cleanCmd, []string{})

		// Assert
		require.Error(t, err)
		assert.Len(t, cleaned, 1)
	})

	t.Run("all flag removes all prefixes of tool", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func() ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Prefix: "agentic", Tool: "claude"},
				{Image: "work-claude", Prefix: "work", Tool: "claude"},
				{Image: "agentic-copilot", Prefix: "agentic", Tool: "copilot"},
			}, nil
		})
		var cleaned []string
		stubCleanImage(t, func(image string) error {
			cleaned = append(cleaned, image)
			return nil
		})

		require.NoError(t, cleanCmd.Flags().Set("all", "true"))
		defer cleanCmd.Flags().Set("all", "false") //nolint:errcheck

		// Act
		err := runClean(cleanCmd, []string{"claude"})

		// Assert
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"agentic-claude", "work-claude"}, cleaned)
	})

	t.Run("all flag no tool removes everything", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func() ([]*docker.ImageInfo, error) {
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

		require.NoError(t, cleanCmd.Flags().Set("all", "true"))
		defer cleanCmd.Flags().Set("all", "false") //nolint:errcheck

		// Act
		err := runClean(cleanCmd, []string{})

		// Assert
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"agentic-claude", "work-claude"}, cleaned)
		assert.True(t, basesCleaned)
	})
}
