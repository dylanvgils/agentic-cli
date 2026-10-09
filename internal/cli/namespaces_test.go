package cli

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

func Test_runNamespacesPrune(t *testing.T) {
	// Keep the repo's own .agenticrc.toml out of the test
	t.Chdir(t.TempDir())

	stubFlag(t, namespacesPruneCmd, "namespace", "agentic")
	oneImage := func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
		return []*docker.ImageInfo{{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"}}, nil
	}

	t.Run("confirmed y removes the namespace's images", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, oneImage)
		var cleaned []string
		stubCleanImage(t, func(image string) error { cleaned = append(cleaned, image); return nil })
		stubStdin(t, "y\n")
		stubErrLog(t)

		// Act
		err := runNamespacesPrune(namespacesPruneCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"agentic-claude"}, cleaned)
	})

	t.Run("declined n removes nothing", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, oneImage)
		var cleanCalled bool
		stubCleanImage(t, func(string) error { cleanCalled = true; return nil })
		stubStdin(t, "n\n")
		stubErrLog(t)

		// Act
		err := runNamespacesPrune(namespacesPruneCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.False(t, cleanCalled)
	})

	t.Run("lists only the resolved namespace", func(t *testing.T) {
		// Arrange
		var capturedFilters []docker.ImageFilter
		stubListAllImages(t, func(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			capturedFilters = filters
			return nil, nil
		})
		stubErrLog(t)

		// Act
		err := runNamespacesPrune(namespacesPruneCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []docker.ImageFilter{docker.NamespaceFilter("agentic")}, capturedFilters)
	})

	t.Run("empty namespace says so without prompting", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) { return nil, nil })
		logBuf := stubErrLog(t)

		// Act
		err := runNamespacesPrune(namespacesPruneCmd, nil)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, logBuf.String(), `agentic: no images found in namespace "agentic"`)
		assert.NotContains(t, logBuf.String(), "[y/N]")
	})

	t.Run("list error propagates", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, fmt.Errorf("docker daemon not running")
		})

		// Act
		err := runNamespacesPrune(namespacesPruneCmd, nil)

		// Assert
		require.ErrorContains(t, err, "docker daemon not running")
	})
}

func Test_removeNamespaceImages(t *testing.T) {
	images := []*docker.ImageInfo{
		{Image: "agentic-claude", Namespace: "agentic", Tool: "claude"},
		{Image: "agentic-copilot", Namespace: "agentic", Tool: "copilot"},
	}

	t.Run("removes each image after one summary line", func(t *testing.T) {
		// Arrange
		logBuf := stubErrLog(t)
		var cleaned []string
		stubCleanImage(t, func(image string) error { cleaned = append(cleaned, image); return nil })

		// Act
		err := removeNamespaceImages("agentic", images)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"agentic-claude", "agentic-copilot"}, cleaned)
		assert.Contains(t, logBuf.String(), `agentic: removing 2 image(s) in namespace "agentic"`)
	})

	t.Run("stops at the first remove error", func(t *testing.T) {
		// Arrange
		stubErrLog(t)
		var cleaned []string
		stubCleanImage(t, func(image string) error {
			cleaned = append(cleaned, image)
			return fmt.Errorf("remove failed")
		})

		// Act
		err := removeNamespaceImages("agentic", images)

		// Assert
		require.ErrorContains(t, err, "remove failed")
		assert.Equal(t, []string{"agentic-claude"}, cleaned)
	})
}

func Test_uniqueNamespaces(t *testing.T) {
	// Arrange
	images := []*docker.ImageInfo{
		{Namespace: "work"},
		{Namespace: ""},
		{Namespace: "agentic"},
		{Namespace: "work"},
	}

	// Act
	namespaces := uniqueNamespaces(images)

	// Assert - first-seen order, no empty or repeated names
	assert.Equal(t, []string{"work", "agentic"}, namespaces)
}
