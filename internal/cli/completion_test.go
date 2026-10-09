package cli

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func Test_builtToolNamesFunc(t *testing.T) {
	// Keep the repo's own .agenticrc.toml out of the test
	t.Chdir(t.TempDir())

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
		stubInspectImage(t, nil, assert.AnError)

		// Act
		names, directive := builtToolNamesFunc(&cobra.Command{}, nil, "")

		// Assert
		assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
		assert.Empty(t, names)
	})

	t.Run("some built", func(t *testing.T) {
		// Arrange
		t.Chdir(t.TempDir())
		stubDocker(t, &fakeDocker{inspectImage: func(name string) (*docker.ImageInfo, error) {
			if name == "agentic-claude" {
				return &docker.ImageInfo{Image: name, ID: "abc"}, nil
			}
			return nil, nil
		}})

		// Act
		names, directive := builtToolNamesFunc(&cobra.Command{}, nil, "")

		// Assert
		assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
		assert.Equal(t, []string{"claude"}, names)
	})
}

func Test_namespacesFunc(t *testing.T) {
	t.Run("returns unique namespaces", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "default-claude", Namespace: "default", Tool: "claude"},
				{Image: "default-copilot", Namespace: "default", Tool: "copilot"},
				{Image: "myproject-claude", Namespace: "myproject", Tool: "claude"},
			}, nil
		})

		// Act
		names, directive := namespacesFunc(nil, nil, "")

		// Assert
		assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
		assert.Equal(t, []string{"default", "myproject"}, names)
	})

	t.Run("skips empty namespace", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return []*docker.ImageInfo{
				{Image: "default-claude", Namespace: "", Tool: "claude"},
				{Image: "myproject-claude", Namespace: "myproject", Tool: "claude"},
			}, nil
		})

		// Act
		names, directive := namespacesFunc(nil, nil, "")

		// Assert
		assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
		assert.Equal(t, []string{"myproject"}, names)
	})

	t.Run("list error returns empty", func(t *testing.T) {
		// Arrange
		stubListAllImages(t, func(...docker.ImageFilter) ([]*docker.ImageInfo, error) {
			return nil, assert.AnError
		})

		// Act
		names, directive := namespacesFunc(nil, nil, "")

		// Assert
		assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
		assert.Empty(t, names)
	})
}

func Test_volumeNamesFunc(t *testing.T) {
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

func Test_dockerContextsFunc(t *testing.T) {
	t.Run("returns context names", func(t *testing.T) {
		// Arrange
		stubListContexts(t, func() ([]string, error) { return []string{"default", "prod"}, nil })

		// Act
		names, directive := dockerContextsFunc(&cobra.Command{}, nil, "")

		// Assert
		assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
		assert.Equal(t, []string{"default", "prod"}, names)
	})

	t.Run("list error returns empty", func(t *testing.T) {
		// Arrange
		stubListContexts(t, func() ([]string, error) { return nil, assert.AnError })

		// Act
		names, directive := dockerContextsFunc(&cobra.Command{}, nil, "")

		// Assert
		assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
		assert.Empty(t, names)
	})
}

func Test_baseLayersFunc(t *testing.T) {
	t.Run("suggests every extra layer", func(t *testing.T) {
		// Act
		names, directive := baseLayersFunc(nil, nil, "")

		// Assert
		assert.Equal(t, tools.KnownExtras(), names)
		assert.Equal(t, cobra.ShellCompDirectiveNoFileComp|cobra.ShellCompDirectiveNoSpace, directive)
	})

	t.Run("after a comma suggests the remaining layers with the listed prefix", func(t *testing.T) {
		// Act
		names, _ := baseLayersFunc(nil, nil, "node,java,")

		// Assert
		assert.Equal(t, []string{"node,java,docker", "node,java,dotnet", "node,java,go"}, names)
	})
}
