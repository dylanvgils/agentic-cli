package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrentContext(t *testing.T) {
	t.Run("explicit context skips docker", func(t *testing.T) {
		// Arrange
		client := &Client{context: "prod", runner: &fakeRunner{}}
		get := stubDockerRunCapture(t, client)

		// Act
		name, err := client.CurrentContext()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "prod", name)
		assert.Empty(t, get())
	})

	t.Run("empty context asks docker and trims output", func(t *testing.T) {
		// Arrange
		client := newTestClient()
		var gotArgs []string
		stubDockerRun(t, client, func(args ...string) (string, error) {
			gotArgs = args
			return "desktop-linux\n", nil
		})

		// Act
		name, err := client.CurrentContext()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"context", "show"}, gotArgs)
		assert.Equal(t, "desktop-linux", name)
	})

	t.Run("propagates error", func(t *testing.T) {
		// Arrange
		client := newTestClient()
		stubDockerRunCapture(t, client, "context show")

		// Act
		_, err := client.CurrentContext()

		// Assert
		assert.Error(t, err)
	})
}

func TestListContexts(t *testing.T) {
	client := newTestClient()

	t.Run("calls docker with format", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client)

		// Act
		_, err := client.ListContexts()

		// Assert
		require.NoError(t, err)
		calls := get()
		require.Len(t, calls, 1)
		assert.Equal(t, []string{"context", "ls", "--format={{.Name}}"}, calls[0].args)
	})

	t.Run("splits lines", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "default\nprod\n", nil)

		// Act
		names, err := client.ListContexts()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"default", "prod"}, names)
	})

	t.Run("empty output returns empty", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", nil)

		// Act
		names, err := client.ListContexts()

		// Assert
		require.NoError(t, err)
		assert.Empty(t, names)
	})

	t.Run("propagates error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, client, "context ls")

		// Act
		_, err := client.ListContexts()

		// Assert
		assert.Error(t, err)
	})
}
