package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNames(t *testing.T) {
	names := Names()

	t.Run("sorted", func(t *testing.T) {
		// Assert
		assert.Equal(t, []string{"claude", "copilot", "opencode"}, names)
	})

	t.Run("match config keys", func(t *testing.T) {
		// Assert
		assert.Len(t, names, len(Configs))
		for _, n := range names {
			assert.Contains(t, Configs, n)
		}
	})
}

func TestImageName(t *testing.T) {
	t.Run("known tool with default prefix", func(t *testing.T) {
		// Act
		image, err := ImageName("claude", DefaultPrefix)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude", image)
	})

	t.Run("known tool with custom prefix", func(t *testing.T) {
		// Act
		image, err := ImageName("claude", "myproject")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "myproject-claude", image)
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		_, err := ImageName("bogus", DefaultPrefix)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bogus")
	})
}

func TestVersionScript_returnsAgenticPrefixedName(t *testing.T) {
	// Act
	result := versionScript("node")

	// Assert
	assert.Equal(t, "agentic-version-node", result)
}
