package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNames_sorted(t *testing.T) {
	// Act
	names := Names()

	// Assert
	assert.Equal(t, []string{"claude", "copilot", "opencode"}, names)
}

func TestNames_matchConfigKeys(t *testing.T) {
	// Act
	names := Names()

	// Assert
	assert.Len(t, names, len(Configs))
	for _, n := range names {
		assert.Contains(t, Configs, n)
	}
}

func TestImageName_knownTool_returnsImageName(t *testing.T) {
	// Act
	image, err := ImageName("claude")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "agentic-claude", image)
}

func TestImageName_unknownTool_returnsError(t *testing.T) {
	// Act
	_, err := ImageName("bogus")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bogus")
}

func TestVersionScript_returnsAgenticPrefixedName(t *testing.T) {
	// Act
	result := versionScript("node")

	// Assert
	assert.Equal(t, "agentic-version-node", result)
}
