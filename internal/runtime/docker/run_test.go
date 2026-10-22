package docker

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_StartSidecars(t *testing.T) {
	client := newTestClient()

	t.Run("refuses a spec without a host user before touching Docker", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client)

		// Act
		_, err := client.StartSidecars(sandbox.RunSpec{Image: "agentic-claude"})

		// Assert
		assert.ErrorIs(t, err, errNoHostUser)
		assert.Empty(t, get())
	})

	t.Run("names the tool container after its image", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, client)

		// Act
		sidecars, err := client.StartSidecars(sandbox.RunSpec{Image: "agentic-claude", User: &testUser})

		// Assert
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(sidecars.run.container, "agentic-claude-"))
		id := strings.TrimPrefix(sidecars.run.container, "agentic-claude-")
		assert.Equal(t, "agentic-run-"+id, sidecars.run.network, "the network shares the container's id")
	})

	t.Run("a sidecar that fails to start returns its error", func(t *testing.T) {
		// Arrange
		stubDindReadyTimeout(t, client)
		stubDockerRunCapture(t, client, "exec")
		rs := sandbox.RunSpec{Image: "agentic-claude", User: &testUser, ToolHome: t.TempDir(), Dind: sandbox.DindSpec{Enabled: true}}

		// Act
		_, err := client.StartSidecars(rs)

		// Assert
		assert.ErrorContains(t, err, "docker sidecar")
	})
}

func TestClient_RunTool(t *testing.T) {
	client := newTestClient()

	get := stubRunInteractive(t, client)
	stubDockerRunCapture(t, client)

	t.Run("security args", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{Image: "agentic-claude"}

		// Act
		err := startAndRunTool(client, rs)

		// Assert
		require.NoError(t, err)
		args := get()
		assert.Contains(t, args, "run")
		assert.Contains(t, args, "--rm")
		assert.Contains(t, args, "--read-only")
		assert.Contains(t, args, "--cap-drop=ALL")
		assert.Contains(t, args, "--security-opt=no-new-privileges:true")
		assert.Contains(t, args, "--user="+testUser.String())
		assert.True(t, hasArgWithPrefix(args, "--network=agentic-run-"), "even a plain run gets its own network")
		assert.False(t, hasArg(args, "--network="+NetworkName), "runs must not share a network")
	})

	t.Run("names the tool container after its image", func(t *testing.T) {
		// Act
		err := startAndRunTool(client, sandbox.RunSpec{Image: "agentic-claude"})

		// Assert
		require.NoError(t, err)
		assert.True(t, hasArgWithPrefix(get(), "--name=agentic-claude-"))
	})

	t.Run("proxy mode keeps the tool off the egress network and injects proxy env", func(t *testing.T) {
		// Arrange
		stubProxyDocker(t, client, "true")
		rs := sandbox.RunSpec{
			Image: "agentic-claude",
			Proxy: sandbox.ProxySpec{Mode: sandbox.ProxyEnforce, Rules: []sandbox.AllowRule{{Host: "api.anthropic.com"}}, LogFile: filepath.Join(t.TempDir(), "proxy_abc.jsonl")},
		}

		// Act
		err := startAndRunTool(client, rs)

		// Assert
		require.NoError(t, err)
		args := get()
		assert.True(t, hasArgWithPrefix(args, "--network=agentic-run-"), "tool should attach to the internal run net")
		assert.False(t, hasArg(args, "--network="+NetworkName), "tool should not be on the egress net directly")
		assert.True(t, hasArgWithPrefix(args, "--env=HTTPS_PROXY="), "tool should get HTTPS_PROXY")
	})

	t.Run("dind mode points tool at sidecar without relaxing tool hardening", func(t *testing.T) {
		// Arrange
		stubDindReadyTimeout(t, client)
		stubDockerRunCapture(t, client)
		rs := sandbox.RunSpec{Image: "agentic-claude", User: &testUser, ToolHome: t.TempDir(), Dind: sandbox.DindSpec{Enabled: true}}

		// Act
		err := startAndRunTool(client, rs)

		// Assert
		require.NoError(t, err)
		args := get()
		assert.True(t, hasArgWithPrefix(args, "--network=agentic-run-"), "tool should share the run net with dind")
		assert.Contains(t, args, "--env=DOCKER_HOST=tcp://agentic-docker:2376")
		assert.Contains(t, args, "--cap-drop=ALL")
		assert.Contains(t, args, "--security-opt=no-new-privileges:true")
		assert.Contains(t, args, "--read-only")
		assert.NotContains(t, args, "--privileged")
		assert.False(t, hasArgWithPrefix(args, "--cap-add"), "tool container must not gain capabilities")
	})

	t.Run("dind with proxy shares the internal network and bypasses the proxy for the daemon", func(t *testing.T) {
		// Arrange
		stubDindReadyTimeout(t, client)
		stubProxyDocker(t, client, "true")
		rs := sandbox.RunSpec{
			Image:    "agentic-claude",
			ToolHome: t.TempDir(),
			Proxy:    sandbox.ProxySpec{Mode: sandbox.ProxyEnforce, LogFile: filepath.Join(t.TempDir(), "proxy_abc.jsonl")},
			Dind:     sandbox.DindSpec{Enabled: true},
		}

		// Act
		err := startAndRunTool(client, rs)

		// Assert
		require.NoError(t, err)
		args := get()
		assert.True(t, hasArgWithPrefix(args, "--network=agentic-run-"))
		assert.Contains(t, args, "--env=NO_PROXY=localhost,127.0.0.1,agentic-docker")
	})

	t.Run("tmpfs mounts", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{
			Image:         "agentic-claude",
			EphemeralDirs: []sandbox.EphemeralDir{{Path: "/tmp", Size: "1g"}},
		}

		// Act
		err := startAndRunTool(client, rs)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, get(), "--tmpfs=/tmp:exec,size=1g")
	})

	t.Run("tmpfs mounts expand container home", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{
			Image:         "agentic-copilot",
			ContainerHome: "/home/user",
			EphemeralDirs: []sandbox.EphemeralDir{{Path: "/tmp", Size: "1g"}, {Path: "$CONTAINER_HOME/.cache", Size: "1g"}},
		}

		// Act
		err := startAndRunTool(client, rs)

		// Assert
		require.NoError(t, err)
		args := get()
		assert.Contains(t, args, "--tmpfs=/tmp:exec,size=1g")
		assert.Contains(t, args, "--tmpfs=/home/user/.cache:exec,size=1g")
	})

	t.Run("image and tool args", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{Image: "agentic-claude", ToolArgs: []string{"--resume"}}

		// Act
		err := startAndRunTool(client, rs)

		// Assert
		require.NoError(t, err)
		args := get()
		n := len(args)
		require.GreaterOrEqual(t, n, 2)
		assert.Equal(t, "agentic-claude", args[n-2])
		assert.Equal(t, "--resume", args[n-1])
	})

	t.Run("skip entrypoint", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{
			Image:          "agentic-claude",
			SkipEntrypoint: true,
		}

		// Act
		err := startAndRunTool(client, rs)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "", argAfter(get(), "--entrypoint"))
	})

	t.Run("volumes", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{
			Image:    "agentic-claude",
			ToolHome: "/home/.agentic",
			Volumes:  []string{"/host:/container", "$TOOL_HOME/data:/data"},
		}

		// Act
		err := startAndRunTool(client, rs)

		// Assert
		require.NoError(t, err)
		args := get()
		assert.Contains(t, args, "--volume=/host:/container")
		assert.Contains(t, args, "--volume=/home/.agentic/data:/data")
	})

	t.Run("secrets", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{
			Image:   "agentic-copilot",
			Secrets: []string{"mytoken:/tmp/token"},
		}

		// Act
		err := startAndRunTool(client, rs)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, get(), "--volume=/tmp/token:/run/secrets/mytoken:ro")
	})

	t.Run("secrets tilde expanded", func(t *testing.T) {
		// Arrange
		home, err := os.UserHomeDir()
		require.NoError(t, err)

		rs := sandbox.RunSpec{
			Image:   "agentic-copilot",
			Secrets: []string{"mytoken:~/secrets/token"},
		}

		// Act
		err = startAndRunTool(client, rs)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, get(), "--volume="+home+"/secrets/token:/run/secrets/mytoken:ro")
	})

	t.Run("secrets dollar HOME expanded", func(t *testing.T) {
		// Arrange
		home, err := os.UserHomeDir()
		require.NoError(t, err)

		rs := sandbox.RunSpec{
			Image:   "agentic-copilot",
			Secrets: []string{"mytoken:$HOME/secrets/token"},
		}

		// Act
		err = startAndRunTool(client, rs)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, get(), "--volume="+home+"/secrets/token:/run/secrets/mytoken:ro")
	})

	t.Run("secrets dollar HOME braces expanded", func(t *testing.T) {
		// Arrange
		home, err := os.UserHomeDir()
		require.NoError(t, err)

		rs := sandbox.RunSpec{
			Image:   "agentic-copilot",
			Secrets: []string{"mytoken:${HOME}/secrets/token"},
		}

		// Act
		err = startAndRunTool(client, rs)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, get(), "--volume="+home+"/secrets/token:/run/secrets/mytoken:ro")
	})

	t.Run("secrets with explicit container path", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{
			Image:   "agentic-copilot",
			Secrets: []string{"maven-settings:/tmp/settings.xml:/root/.m2/settings.xml"},
		}

		// Act
		err := startAndRunTool(client, rs)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, get(), "--volume=/tmp/settings.xml:/root/.m2/settings.xml:ro")
	})

	t.Run("secrets with $CONTAINER_HOME in container path", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{
			Image:         "agentic-copilot",
			ContainerHome: "/root",
			Secrets:       []string{"maven-settings:/tmp/settings.xml:$CONTAINER_HOME/.m2/settings.xml"},
		}

		// Act
		err := startAndRunTool(client, rs)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, get(), "--volume=/tmp/settings.xml:/root/.m2/settings.xml:ro")
	})

	t.Run("secrets empty target path", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{
			Image:   "agentic-copilot",
			Secrets: []string{"mytoken:/tmp/token:"},
		}

		// Act + Assert
		assert.ErrorContains(t, startAndRunTool(client, rs), "empty target path")
	})

	t.Run("secrets invalid format", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{
			Image:   "agentic-copilot",
			Secrets: []string{"badformat"},
		}

		// Act + Assert
		assert.ErrorContains(t, startAndRunTool(client, rs), "invalid secret")
	})
}

func TestClient_StopSidecars(t *testing.T) {
	client := newTestClient()

	t.Run("removes the sidecars it started", func(t *testing.T) {
		// Arrange
		stubDindReadyTimeout(t, client)
		calls := stubDockerRunCapture(t, client)
		rs := sandbox.RunSpec{Image: "agentic-claude", User: &testUser, ToolHome: t.TempDir(), Dind: sandbox.DindSpec{Enabled: true}}
		sidecars, err := client.StartSidecars(rs)
		require.NoError(t, err)

		// Act
		client.StopSidecars(sidecars)

		// Assert
		rm := findCall(calls(), "rm", "--force", "--volumes")
		require.NotNil(t, rm)
		assert.True(t, hasArgWithPrefix(rm, dindHostAlias+"-"))
	})

	t.Run("zero value does nothing", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client)

		// Act
		client.StopSidecars(Sidecars{})

		// Assert
		assert.Empty(t, calls())
	})
}

func Test_Client_attachTool(t *testing.T) {
	client := newTestClient()

	t.Run("removes the tool container once docker run returns", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client)
		stubRunInteractive(t, client)

		// Act
		err := client.attachTool("agentic-claude-test", []string{"run"})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []dockerCall{{args: []string{"rm", "--force", "agentic-claude-test"}}}, calls())
	})

	t.Run("removes the tool container when docker run fails", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client)
		stubRunInteractiveFunc(t, client, func(...string) error { return errors.New("stub: docker run detached") })

		// Act
		err := client.attachTool("agentic-claude-test", []string{"run"})

		// Assert
		require.EqualError(t, err, "stub: docker run detached")
		assert.Equal(t, []dockerCall{{args: []string{"rm", "--force", "agentic-claude-test"}}}, calls())
	})
}
