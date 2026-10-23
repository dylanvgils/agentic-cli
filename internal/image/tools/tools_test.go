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
	t.Run("known tool with default namespace", func(t *testing.T) {
		// Act
		image, err := ImageName("claude", "agentic")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude", image)
	})

	t.Run("known tool with custom namespace", func(t *testing.T) {
		// Act
		image, err := ImageName("claude", "myproject")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "myproject-claude", image)
	})

	t.Run("unknown tool returns error", func(t *testing.T) {
		// Act
		_, err := ImageName("bogus", "agentic")

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

func TestParseImageName(t *testing.T) {
	t.Run("default namespace and known tool", func(t *testing.T) {
		// Act
		namespace, tool, ok := ParseImageName("agentic-claude")

		// Assert
		assert.True(t, ok)
		assert.Equal(t, "agentic", namespace)
		assert.Equal(t, "claude", tool)
	})

	t.Run("custom namespace and known tool", func(t *testing.T) {
		// Act
		namespace, tool, ok := ParseImageName("myproject-copilot")

		// Assert
		assert.True(t, ok)
		assert.Equal(t, "myproject", namespace)
		assert.Equal(t, "copilot", tool)
	})

	t.Run("multi-segment namespace", func(t *testing.T) {
		// Act
		namespace, tool, ok := ParseImageName("my-long-project-opencode")

		// Assert
		assert.True(t, ok)
		assert.Equal(t, "my-long-project", namespace)
		assert.Equal(t, "opencode", tool)
	})

	t.Run("unknown tool returns false", func(t *testing.T) {
		// Act
		_, _, ok := ParseImageName("agentic-bogus")

		// Assert
		assert.False(t, ok)
	})

	t.Run("no dash returns false", func(t *testing.T) {
		// Act
		_, _, ok := ParseImageName("claude")

		// Assert
		assert.False(t, ok)
	})
}
