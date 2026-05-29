package cmd

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestBuiltToolNamesFunc(t *testing.T) {
	t.Run("all built", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{Image: "agentic-claude", ID: "abc"}, nil)

		// Act
		names, directive := builtToolNamesFunc(&cobra.Command{}, nil, "")

		// Assert
		assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
		assert.Equal(t, []string{"claude", "copilot", "opencode"}, names)
	})

	t.Run("tool already provided", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, &docker.ImageInfo{Image: "agentic-claude", ID: "abc"}, nil)

		// Act
		names, directive := builtToolNamesFunc(&cobra.Command{}, []string{"claude"}, "")

		// Assert
		assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
		assert.Empty(t, names)
	})

	t.Run("none built", func(t *testing.T) {
		// Arrange
		stubInspectImage(t, nil, nil)

		// Act
		names, directive := builtToolNamesFunc(&cobra.Command{}, nil, "")

		// Assert
		assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
		assert.Empty(t, names)
	})

	t.Run("inspect error", func(t *testing.T) {
		// Arrange
		orig := inspectImage
		inspectImage = func(_ string) (*docker.ImageInfo, error) {
			return nil, assert.AnError
		}
		defer func() { inspectImage = orig }()

		// Act
		names, directive := builtToolNamesFunc(&cobra.Command{}, nil, "")

		// Assert
		assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
		assert.Empty(t, names)
	})

	t.Run("some built", func(t *testing.T) {
		// Arrange
		orig := inspectImage
		inspectImage = func(name string) (*docker.ImageInfo, error) {
			if name == "agentic-claude" {
				return &docker.ImageInfo{Image: name, ID: "abc"}, nil
			}
			return nil, nil
		}
		defer func() { inspectImage = orig }()

		// Act
		names, directive := builtToolNamesFunc(&cobra.Command{}, nil, "")

		// Assert
		assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
		assert.Equal(t, []string{"claude"}, names)
	})
}

func TestVolumeNamesFunc(t *testing.T) {
	t.Run("returns volume names", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) { return []string{"maven", "gradle"}, nil })

		// Act
		names, directive := volumeNamesFunc(&cobra.Command{}, nil, "")

		// Assert
		assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
		assert.Equal(t, []string{"maven", "gradle"}, names)
	})

	t.Run("arg already provided returns empty", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) { return []string{"maven"}, nil })

		// Act
		names, directive := volumeNamesFunc(&cobra.Command{}, []string{"maven"}, "")

		// Assert
		assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
		assert.Empty(t, names)
	})

	t.Run("list error returns empty", func(t *testing.T) {
		// Arrange
		stubListVolumeNames(t, func() ([]string, error) { return nil, assert.AnError })

		// Act
		names, directive := volumeNamesFunc(&cobra.Command{}, nil, "")

		// Assert
		assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
		assert.Empty(t, names)
	})
}
