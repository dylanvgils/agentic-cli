package cmd

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestBuiltToolNamesFunc_AllBuilt(t *testing.T) {
	restore := stubInspectImage(t, &docker.ImageInfo{Image: "agentic-claude", ID: "abc"}, nil)
	defer restore()

	// Act
	names, directive := builtToolNamesFunc(&cobra.Command{}, nil, "")

	// Assert
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Equal(t, []string{"claude", "copilot", "opencode"}, names)
}

func TestBuiltToolNamesFunc_ToolAlreadyProvided(t *testing.T) {
	restore := stubInspectImage(t, &docker.ImageInfo{Image: "agentic-claude", ID: "abc"}, nil)
	defer restore()

	// Act
	names, directive := builtToolNamesFunc(&cobra.Command{}, []string{"claude"}, "")

	// Assert
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Empty(t, names)
}

func TestBuiltToolNamesFunc_NoneBuilt(t *testing.T) {
	restore := stubInspectImage(t, nil, nil)
	defer restore()

	// Act
	names, directive := builtToolNamesFunc(&cobra.Command{}, nil, "")

	// Assert
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Empty(t, names)
}

func TestBuiltToolNamesFunc_InspectError(t *testing.T) {
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
}

func TestBuiltToolNamesFunc_SomeBuilt(t *testing.T) {
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
}

func TestVolumeNamesFunc_returnsVolumeNames(t *testing.T) {
	restore := stubListVolumeNames(t, func() ([]string, error) { return []string{"maven", "gradle"}, nil })
	defer restore()

	// Act
	names, directive := volumeNamesFunc(&cobra.Command{}, nil, "")

	// Assert
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Equal(t, []string{"maven", "gradle"}, names)
}

func TestVolumeNamesFunc_argAlreadyProvided_returnsEmpty(t *testing.T) {
	restore := stubListVolumeNames(t, func() ([]string, error) { return []string{"maven"}, nil })
	defer restore()

	// Act
	names, directive := volumeNamesFunc(&cobra.Command{}, []string{"maven"}, "")

	// Assert
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Empty(t, names)
}

func TestVolumeNamesFunc_listError_returnsEmpty(t *testing.T) {
	restore := stubListVolumeNames(t, func() ([]string, error) { return nil, assert.AnError })
	defer restore()

	// Act
	names, directive := volumeNamesFunc(&cobra.Command{}, nil, "")

	// Assert
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Empty(t, names)
}
