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

func TestClient_requireLocalContext(t *testing.T) {
	stubHost := func(t *testing.T, host string) *Client {
		t.Helper()
		client := &Client{context: "example", runner: &fakeRunner{}}
		stubDockerRunFixed(t, client, host+"\n", nil)
		return client
	}

	t.Run("accepts a local socket or pipe", func(t *testing.T) {
		for _, host := range []string{"unix:///var/run/docker.sock", "npipe:////./pipe/docker_engine"} {
			// Arrange
			client := stubHost(t, host)

			// Act
			err := client.requireLocalContext()

			// Assert
			assert.NoError(t, err, host)
		}
	})

	t.Run("refuses a remote daemon", func(t *testing.T) {
		for _, host := range []string{"tcp://docker.example.test:2376", "ssh://user@docker.example.test"} {
			// Arrange
			client := stubHost(t, host)

			// Act
			err := client.requireLocalContext()

			// Assert
			assert.ErrorContains(t, err, `docker context "example" (`+host+`) isn't on this machine`)
		}
	})
}
