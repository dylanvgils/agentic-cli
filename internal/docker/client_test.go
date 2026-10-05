package docker

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	// Act
	client := New("prod")

	// Assert
	assert.Equal(t, "prod", client.Context())
	assert.Equal(t, execRunner{}, client.runner)
}

func TestClient_run(t *testing.T) {
	t.Run("captures output", func(t *testing.T) {
		// Arrange
		stubDocker(t, `printf '%s\n' "$@"`)
		client := New("")

		// Act
		out, err := client.run("images", "--quiet")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "images\n--quiet\n", out)
	})

	t.Run("returns error on failure", func(t *testing.T) {
		// Arrange
		stubDocker(t, `exit 1`)
		client := New("")

		// Act
		_, err := client.run("bad-command")

		// Assert
		assert.Error(t, err)
	})

	t.Run("prepends --context to the executed command", func(t *testing.T) {
		// Arrange
		stubDocker(t, `printf '%s\n' "$@"`)
		client := New("prod")

		// Act
		out, err := client.run("images", "--quiet")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "--context\nprod\nimages\n--quiet\n", out)
	})
}

func TestClient_runStdin(t *testing.T) {
	t.Run("pipes stdin", func(t *testing.T) {
		// Arrange
		stubDocker(t, `cat`)
		client := New("")

		// Act
		out, err := client.runStdin(strings.NewReader("hello\n"), "build", "-")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "hello\n", out)
	})

	t.Run("nil stdin does not block", func(t *testing.T) {
		// Arrange
		stubDocker(t, `echo ok`)
		client := New("")

		// Act
		out, err := client.runStdin(nil, "info")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "ok\n", out)
	})
}

func TestClient_runInteractive(t *testing.T) {
	t.Run("prepends --context to the executed command", func(t *testing.T) {
		// Arrange
		client := newTestClient()
		get := stubRunInteractive(t, client)
		client.context = "prod"

		// Act
		err := client.runInteractive("build", "-t", "img")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"--context", "prod", "build", "-t", "img"}, get())
	})
}

func TestClient_withContext(t *testing.T) {
	t.Run("returns args unchanged when context unset", func(t *testing.T) {
		// Arrange
		client := New("")

		// Act
		result := client.withContext([]string{"build", "-t", "img"})

		// Assert
		assert.Equal(t, []string{"build", "-t", "img"}, result)
	})

	t.Run("prepends --context flag when context set", func(t *testing.T) {
		// Arrange
		client := New("prod")

		// Act
		result := client.withContext([]string{"build", "-t", "img"})

		// Assert
		assert.Equal(t, []string{"--context", "prod", "build", "-t", "img"}, result)
	})
}
