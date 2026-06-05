package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_listNamespaces(t *testing.T) {
	t.Run("prints unique namespaces sorted alphabetically", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Namespace: "work", Tool: "claude"},
				{Namespace: "agentic", Tool: "claude"},
			}, nil
		})

		// Act
		out := captureStdout(t, func() {
			err := listNamespaces()
			require.NoError(t, err)
		})

		// Assert
		assert.Less(t, strings.Index(out, "agentic"), strings.Index(out, "work"))
	})

	t.Run("deduplicates namespaces from multiple images", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Namespace: "agentic", Tool: "claude"},
				{Namespace: "agentic", Tool: "copilot"},
			}, nil
		})

		// Act
		out := captureStdout(t, func() {
			err := listNamespaces()
			require.NoError(t, err)
		})

		// Assert
		assert.Equal(t, 1, strings.Count(out, "agentic"))
	})

	t.Run("empty prints no-images message", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, nil
		})

		// Act
		out := captureStdout(t, func() {
			err := listNamespaces()
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "no agentic images found")
	})

	t.Run("docker error propagates", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, fmt.Errorf("docker daemon not running")
		})

		// Act
		err := listNamespaces()

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})
}

func Test_pruneNamespace(t *testing.T) {
	t.Run("calls cleanImage for each image in the namespace", func(t *testing.T) {
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

		// Act
		err := pruneNamespace("agentic")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"agentic-claude", "agentic-copilot"}, cleaned)
	})

	t.Run("passes namespace filter to listAllImages", func(t *testing.T) {
		// Arrange
		var capturedFilters []docker.ImageFilter
		stubListAllImages(t, func(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			capturedFilters = filters
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
			}, nil
		})
		stubCleanImage(t, func(string) error { return nil })

		// Act
		err := pruneNamespace("agentic")
		require.NoError(t, err)

		// Assert
		assert.Equal(t, []docker.ImageFilter{docker.NamespaceFilter("agentic")}, capturedFilters)
	})

	t.Run("empty namespace prints message without error", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, nil
		})

		// Act
		out := captureStdout(t, func() {
			err := pruneNamespace("agentic")
			require.NoError(t, err)
		})

		// Assert
		assert.Contains(t, out, "no images found in namespace")
	})

	t.Run("docker list error propagates", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, fmt.Errorf("docker daemon not running")
		})

		// Act
		err := pruneNamespace("agentic")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "docker daemon not running")
	})

	t.Run("cleanImage error propagates", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
			}, nil
		})
		stubCleanImage(t, func(string) error { return fmt.Errorf("remove failed") })

		// Act
		err := pruneNamespace("agentic")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "remove failed")
	})
}
