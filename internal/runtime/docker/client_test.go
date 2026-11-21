package docker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/infra/cmdlog"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	// Arrange
	log := cmdlog.New()

	// Act
	client := New("prod", log)

	// Assert
	assert.Equal(t, "prod", client.Context())
	assert.Equal(t, execRunner{log: log}, client.runner)
	assert.Equal(t, defaultTiming(), client.timing)
}

func TestClient_Capabilities(t *testing.T) {
	// Act
	caps := New("", cmdlog.New()).Capabilities()

	// Assert
	assert.Equal(t, sandbox.Capabilities{
		Features: sandbox.Features{
			Proxy:               true,
			ProxyMonitor:        true,
			Dind:                true,
			DindLimits:          true,
			CredentialInjection: true,
			PathRules:           true,
			ProxyIntegrations:   true,
			PidsLimit:           true,
		},
		Isolation: sandbox.Isolation{
			Boundary:            sandbox.BoundaryContainer,
			ReadOnlyRoot:        true,
			DropsPrivileges:     true,
			CredentialsViaProxy: true,
			EgressViaProxy:      true,
		},
	}, caps)
}

func TestClient_run(t *testing.T) {
	for _, tc := range []struct {
		name    string
		context string
		want    string
	}{
		{
			name:    "captures output",
			context: "",
			want:    "images\n--quiet\n",
		},
		{
			name:    "prepends --context to the executed command",
			context: "prod",
			want:    "--context\nprod\nimages\n--quiet\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			stubDocker(t, `printf '%s\n' "$@"`)
			client := New(tc.context, cmdlog.New())

			// Act
			out, err := client.run("images", "--quiet")

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, out)
		})
	}

	t.Run("returns error on failure", func(t *testing.T) {
		// Arrange
		stubDocker(t, `exit 1`)
		client := New("", cmdlog.New())

		// Act
		_, err := client.run("bad-command")

		// Assert
		assert.Error(t, err)
	})
}

func TestClient_runWithStdin(t *testing.T) {
	t.Run("pipes stdin", func(t *testing.T) {
		// Arrange
		stubDocker(t, `cat`)
		client := New("", cmdlog.New())

		// Act
		out, err := client.runWithStdin(strings.NewReader("hello\n"), "build", "-")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "hello\n", out)
	})

	t.Run("prepends --context to the executed command", func(t *testing.T) {
		// Arrange
		stubDocker(t, `printf '%s\n' "$@"`)
		client := New("prod", cmdlog.New())

		// Act
		out, err := client.runWithStdin(strings.NewReader(""), "build", "-")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "--context\nprod\nbuild\n-\n", out)
	})

	t.Run("returns error with output on failure", func(t *testing.T) {
		// Arrange
		stubDocker(t, `echo boom; exit 1`)
		client := New("", cmdlog.New())

		// Act
		_, err := client.runWithStdin(strings.NewReader(""), "build", "-")

		// Assert
		assert.ErrorContains(t, err, "boom")
	})
}

func TestClient_runInteractive(t *testing.T) {
	// Arrange
	client := newTestClient()
	get := stubRunInteractive(t, client)
	client.context = "prod"

	// Act
	err := client.runInteractive("run", "img")

	// Assert - attached to the terminal, not as progress
	require.NoError(t, err)
	assert.Equal(t, []string{"--context", "prod", "run", "img"}, get())
	assert.False(t, fakeOf(client).progress)
}

func TestClient_runProgress(t *testing.T) {
	// Arrange
	client := newTestClient()
	get := stubRunInteractive(t, client)
	client.context = "prod"

	// Act
	err := client.runProgress("pull", "img")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"--context", "prod", "pull", "img"}, get())
	assert.True(t, fakeOf(client).progress)
}

func TestClient_stream(t *testing.T) {
	t.Run("writes stdout to the writer", func(t *testing.T) {
		// Arrange
		stubDocker(t, `printf '%s\n' "$@"; echo noise >&2`)
		client := New("prod", cmdlog.New())
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
		client := New("", cmdlog.New())
		var out strings.Builder

		// Act
		err := client.stream(&out, "save", "img")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "boom")
	})
}

func Test_execRunner_run(t *testing.T) {
	// Arrange
	stubDocker(t, `exit 3`)
	path := filepath.Join(t.TempDir(), "commands.jsonl")
	log := cmdlog.New()
	require.NoError(t, log.Open(path))
	t.Cleanup(func() { log.Close() }) //nolint:errcheck

	// Act
	_, err := New("", log).run("run", "--env=API_KEY=test-secret", "img")

	// Assert
	require.Error(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(got), `"command":"docker","args":["run","--env=API_KEY=***","img"],"reason":"Test_execRunner_run","exit_code":3`)
	assert.NotContains(t, string(got), "test-secret")
}

func Test_defaultTiming(t *testing.T) {
	// Act
	timing := defaultTiming()

	// Assert
	assert.Greater(t, timing.sidecarOrphanGrace, timing.dindReadyTimeout, "a starting run's sidecars must outlast its readiness wait")
}
