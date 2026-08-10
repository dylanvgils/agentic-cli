package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListContexts(t *testing.T) {
	t.Run("calls docker with format", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)

		// Act
		_, err := ListContexts()

		// Assert
		require.NoError(t, err)
		calls := get()
		require.Len(t, calls, 1)
		assert.Equal(t, []string{"context", "ls", "--format={{.Name}}"}, calls[0].args)
	})

	t.Run("splits lines", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "default\nprod\n", nil)

		// Act
		names, err := ListContexts()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"default", "prod"}, names)
	})

	t.Run("empty output returns empty", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", nil)

		// Act
		names, err := ListContexts()

		// Assert
		require.NoError(t, err)
		assert.Empty(t, names)
	})

	t.Run("propagates error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, "context ls")

		// Act
		_, err := ListContexts()

		// Assert
		assert.Error(t, err)
	})
}
