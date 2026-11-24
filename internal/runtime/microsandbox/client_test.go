package microsandbox

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
	client := New(log)

	// Assert
	assert.Equal(t, execRunner{log: log}, client.runner)
}

func TestClient_Capabilities(t *testing.T) {
	for _, tc := range []struct {
		name         string
		agenticProxy bool
		want         sandbox.Capabilities
	}{
		{
			name: "native reports msb's own",
			want: sandbox.Capabilities{
				Features: sandbox.Features{
					Proxy:               true,
					Dind:                true,
					CredentialInjection: true,
					PortsWithProxy:      true,
				},
				Isolation: sandbox.Isolation{Boundary: sandbox.BoundaryVM},
			},
		},
		{
			name:         "agentic-proxy adds monitor, path rules, integrations and proxy egress and injection",
			agenticProxy: true,
			want: sandbox.Capabilities{
				Features: sandbox.Features{
					Proxy:               true,
					ProxyMonitor:        true,
					Dind:                true,
					CredentialInjection: true,
					PortsWithProxy:      true,
					PathRules:           true,
					ProxyIntegrations:   true,
				},
				Isolation: sandbox.Isolation{
					Boundary:            sandbox.BoundaryVM,
					CredentialsViaProxy: true,
					EgressViaProxy:      true,
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			caps := New(cmdlog.New()).Capabilities(tc.agenticProxy)

			// Assert
			assert.Equal(t, tc.want, caps)
		})
	}
}

func TestClient_Available(t *testing.T) {
	t.Run("nil when msb is on PATH", func(t *testing.T) {
		// Arrange
		stubMsb(t, "true")

		// Act
		err := New(cmdlog.New()).Available()

		// Assert
		assert.NoError(t, err)
	})

	t.Run("install hint when msb is missing", func(t *testing.T) {
		// Arrange
		t.Setenv("PATH", t.TempDir())

		// Act
		err := New(cmdlog.New()).Available()

		// Assert
		assert.ErrorIs(t, err, errNotInstalled)
	})
}

func TestClient_Version(t *testing.T) {
	t.Run("parses the version from msb --version", func(t *testing.T) {
		// Arrange
		client, args := stubRun("msb 0.2.6\n", nil)

		// Act
		result, err := client.Version()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "0.2.6", result)
		assert.Equal(t, []string{"--version"}, *args)
	})

	t.Run("msb error propagates", func(t *testing.T) {
		// Arrange
		client, _ := stubRun("", errNotInstalled)

		// Act
		_, err := client.Version()

		// Assert
		require.ErrorIs(t, err, errNotInstalled)
	})

	t.Run("output without a version errors", func(t *testing.T) {
		// Arrange
		client, _ := stubRun("msb\n", nil)

		// Act
		_, err := client.Version()

		// Assert
		require.Error(t, err)
	})
}

func Test_execRunner_Run(t *testing.T) {
	t.Run("captures output", func(t *testing.T) {
		// Arrange
		stubMsb(t, `printf '%s\n' "$@"`)

		// Act
		out, err := execRunner{log: cmdlog.New()}.Run("images", "--quiet")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "images\n--quiet\n", out)
	})

	t.Run("returns output in the error on failure", func(t *testing.T) {
		// Arrange
		stubMsb(t, `echo boom; exit 1`)

		// Act
		_, err := execRunner{log: cmdlog.New()}.Run("load")

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "boom")
	})
}

func Test_execRunner_RunWithStdin(t *testing.T) {
	// Arrange
	stubMsb(t, `printf '%s\n' "$@"; cat`)

	// Act
	out, err := execRunner{log: cmdlog.New()}.RunWithStdin(strings.NewReader("tar"), "load")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "load\ntar", out)
}

func Test_execRunner_RunInteractive(t *testing.T) {
	// Arrange
	out := filepath.Join(t.TempDir(), "env")
	stubMsb(t, `echo "$EXAMPLE_TOKEN" > "`+out+`"`)

	// Act
	err := execRunner{log: cmdlog.New()}.RunInteractive([]string{"EXAMPLE_TOKEN=test-secret"}, "run", "img")

	// Assert
	require.NoError(t, err)
	got, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, "test-secret\n", string(got))
}

func Test_execRunner_run(t *testing.T) {
	// Arrange
	stubMsb(t, `exit 0`)
	path := filepath.Join(t.TempDir(), "commands.jsonl")
	log := cmdlog.New()
	require.NoError(t, log.Open(path))
	t.Cleanup(func() { log.Close() }) //nolint:errcheck

	// Act
	err := execRunner{log: log}.RunInteractive([]string{"EXAMPLE_TOKEN=test-secret"}, "run", "--env=API_KEY=test-secret", "img")

	// Assert
	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(got), `"command":"msb","args":["run","--env=API_KEY=***","img"],"exit_code":0`)
	assert.NotContains(t, string(got), "test-secret")
}
