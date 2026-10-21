package docker

import (
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	// Act
	client := New("prod")

	// Assert
	assert.Equal(t, "prod", client.Context())
	assert.Equal(t, execRunner{}, client.runner)
	assert.Equal(t, defaultTiming(), client.timing)
}

func TestClient_Capabilities(t *testing.T) {
	// Act
	caps := New("").Capabilities()

	// Assert
	assert.Equal(t, sandbox.Capabilities{
		Proxy:               true,
		ProxyMonitor:        true,
		Dind:                true,
		CredentialInjection: true,
		PathRules:           true,
		CredentialsViaProxy: true,
		SecretMounts:        true,
		PidsLimit:           true,
		ReadOnlyRoot:        true,
		DropsPrivileges:     true,
	}, caps)
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

func TestClient_runWithStdin(t *testing.T) {
	t.Run("pipes stdin", func(t *testing.T) {
		// Arrange
		stubDocker(t, `cat`)
		client := New("")

		// Act
		out, err := client.runWithStdin(strings.NewReader("hello\n"), "build", "-")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "hello\n", out)
	})

	t.Run("prepends --context to the executed command", func(t *testing.T) {
		// Arrange
		stubDocker(t, `printf '%s\n' "$@"`)
		client := New("prod")

		// Act
		out, err := client.runWithStdin(strings.NewReader(""), "build", "-")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "--context\nprod\nbuild\n-\n", out)
	})

	t.Run("returns error with output on failure", func(t *testing.T) {
		// Arrange
		stubDocker(t, `echo boom; exit 1`)
		client := New("")

		// Act
		_, err := client.runWithStdin(strings.NewReader(""), "build", "-")

		// Assert
		assert.ErrorContains(t, err, "boom")
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

func TestClient_stream(t *testing.T) {
	t.Run("writes stdout to the writer", func(t *testing.T) {
		// Arrange
		stubDocker(t, `printf '%s\n' "$@"; echo noise >&2`)
		client := New("prod")
		var out strings.Builder

		// Act
		err := client.stream(&out, "save", "img")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "--context\nprod\nsave\nimg\n", out.String())
	})

	t.Run("returns stderr in the error on failure", func(t *testing.T) {
		// Arrange
		stubDocker(t, `echo boom >&2; exit 1`)
		client := New("")
		var out strings.Builder

		// Act
		err := client.stream(&out, "save", "img")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "boom")
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

func Test_defaultTiming(t *testing.T) {
	// Act
	timing := defaultTiming()

	// Assert
	assert.Greater(t, timing.sidecarOrphanGrace, timing.dindReadyTimeout, "a starting run's sidecars must outlast its readiness wait")
}
