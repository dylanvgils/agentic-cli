package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_runNamespaces(t *testing.T) {
	t.Run("list: prints unique namespaces sorted alphabetically", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Namespace: "work", Tool: "claude"},
				{Namespace: "agentic", Tool: "claude"},
			}, nil
		})

		// Act
		out := captureStdout(t, func() {
			err := runNamespaces(namespacesCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Less(t, strings.Index(out, "agentic"), strings.Index(out, "work"))
	})

	t.Run("list: deduplicates namespaces from multiple images", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Namespace: "agentic", Tool: "claude"},
				{Namespace: "agentic", Tool: "copilot"},
			}, nil
		})

		// Act
		out := captureStdout(t, func() {
			err := runNamespaces(namespacesCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Equal(t, 1, strings.Count(out, "agentic"))
	})

	t.Run("list: empty prints no-images message", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, nil
		})

		// Act
		out := captureStdout(t, func() {
			err := runNamespaces(namespacesCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "no agentic images found")
	})

	t.Run("list: docker error propagates", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, fmt.Errorf("docker daemon not running")
		})

		// Act
		err := runNamespaces(namespacesCmd, []string{})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})

	t.Run("prune: calls cleanImage for each image in the namespace", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
				{Image: "agentic-copilot", Namespace: "agentic", Tool: "copilot"},
			}, nil
		})
		var cleaned []string
		stubCleanImage(t, func(image string) error {
			cleaned = append(cleaned, image)
			return nil
		})
		require.NoError(t, namespacesCmd.Flags().Set("prune", "true"))
		defer namespacesCmd.Flags().Set("prune", "false") //nolint:errcheck

		// Act
		err := runNamespaces(namespacesCmd, []string{})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"agentic-claude", "agentic-copilot"}, cleaned)
	})

	t.Run("prune: passes namespace filter to listAllImages", func(t *testing.T) {
		// Arrange
		var capturedFilters []docker.ImageFilter
		stubListAllImages(t, func(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			capturedFilters = filters
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
			}, nil
		})
		stubCleanImage(t, func(string) error { return nil })
		require.NoError(t, namespacesCmd.Flags().Set("prune", "true"))
		defer namespacesCmd.Flags().Set("prune", "false") //nolint:errcheck

		// Act
		err := runNamespaces(namespacesCmd, []string{})
		require.NoError(t, err)

		// Assert
		assert.Equal(t, []docker.ImageFilter{docker.NamespaceFilter("agentic")}, capturedFilters)
	})

	t.Run("prune: empty namespace prints message without error", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, nil
		})
		require.NoError(t, namespacesCmd.Flags().Set("prune", "true"))
		defer namespacesCmd.Flags().Set("prune", "false") //nolint:errcheck

		// Act
		out := captureStdout(t, func() {
			err := runNamespaces(namespacesCmd, []string{})
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "no images found in namespace")
	})

	t.Run("prune: docker list error propagates", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, fmt.Errorf("docker daemon not running")
		})
		require.NoError(t, namespacesCmd.Flags().Set("prune", "true"))
		defer namespacesCmd.Flags().Set("prune", "false") //nolint:errcheck

		// Act
		err := runNamespaces(namespacesCmd, []string{})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})

	t.Run("prune: cleanImage error propagates", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
			}, nil
		})
		stubCleanImage(t, func(string) error { return fmt.Errorf("remove failed") })
		require.NoError(t, namespacesCmd.Flags().Set("prune", "true"))
		defer namespacesCmd.Flags().Set("prune", "false") //nolint:errcheck

		// Act
		err := runNamespaces(namespacesCmd, []string{})

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "remove failed")
	})
}
