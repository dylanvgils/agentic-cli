package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_withContext(t *testing.T) {
	t.Run("returns args unchanged when context unset", func(t *testing.T) {
		// Arrange
		SetContext("")
		t.Cleanup(func() { SetContext("") })

		// Act
		result := withContext([]string{"build", "-t", "img"})

		// Assert
		assert.Equal(t, []string{"build", "-t", "img"}, result)
	})

	t.Run("prepends --context flag when context set", func(t *testing.T) {
		// Arrange
		SetContext("prod")
		t.Cleanup(func() { SetContext("") })

		// Act
		result := withContext([]string{"build", "-t", "img"})

		// Assert
		assert.Equal(t, []string{"--context", "prod", "build", "-t", "img"}, result)
	})
}

func TestSetContext(t *testing.T) {
	t.Run("Context reflects the value set", func(t *testing.T) {
		// Arrange
		t.Cleanup(func() { SetContext("") })

		// Act
		SetContext("staging")

		// Assert
		assert.Equal(t, "staging", Context())
	})
}

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
