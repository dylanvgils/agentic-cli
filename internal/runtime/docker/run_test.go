package docker

import (
	"errors"
	"path/filepath"
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
		_, err := client.StartSidecars(sandbox.RunSpec{Image: "agentic-claude"}, "")

		// Assert
		assert.ErrorIs(t, err, errNoHostUser)
		assert.Empty(t, get())
	})

	t.Run("refuses an invalid run id before touching Docker", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client)

		// Act
		_, err := client.StartSidecars(sandbox.RunSpec{Image: "agentic-claude", User: &testUser}, "")

		// Assert
		require.EqualError(t, err, `invalid run id ""`)
		assert.Empty(t, get())
	})

	t.Run("names the tool container and network after the run and keeps the proxy log", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, client)

		// Act
		sidecars, err := client.StartSidecars(sandbox.RunSpec{
			RunID: testRunID,
			Image: "agentic-claude",
			User:  &testUser,
		}, "/example.test/logs/proxy_abc.jsonl")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic-claude-"+string(testRunID), sidecars.run.container)
		assert.Equal(t, "agentic-run-"+string(testRunID), sidecars.run.network)
		assert.Equal(t, "/example.test/logs/proxy_abc.jsonl", sidecars.run.proxyLog)
	})

	t.Run("a sidecar that fails to start returns its error", func(t *testing.T) {
		// Arrange
		stubDindReadyTimeout(t, client)
		stubDockerRunCapture(t, client, "exec")
		rs := sandbox.RunSpec{
			RunID:    testRunID,
			Image:    "agentic-claude",
			User:     &testUser,
			ToolHome: t.TempDir(),
			Dind:     sandbox.DindSpec{Enabled: true},
		}

		// Act
		_, err := client.StartSidecars(rs, "")

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
		err := startAndRunTool(client, rs, "")

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
		assert.True(t, hasArgWithPrefix(args, "--name=agentic-claude-"))
	})

	t.Run("proxy mode keeps the tool off the egress network and injects proxy env", func(t *testing.T) {
		// Arrange
		stubProxyDocker(t, client, "true")
		rs := sandbox.RunSpec{
			Image:  "agentic-claude",
			Egress: sandbox.EgressPolicy{Mode: sandbox.EgressEnforce, Rules: []sandbox.AllowRule{{Host: "api.anthropic.com"}}},
		}

		// Act
		err := startAndRunTool(client, rs, filepath.Join(t.TempDir(), "proxy_abc.jsonl"))

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
		rs := sandbox.RunSpec{
			Image:    "agentic-claude",
			User:     &testUser,
			ToolHome: t.TempDir(),
			Dind:     sandbox.DindSpec{Enabled: true},
		}

		// Act
		err := startAndRunTool(client, rs, "")

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
			Egress:   sandbox.EgressPolicy{Mode: sandbox.EgressEnforce},
			Dind:     sandbox.DindSpec{Enabled: true},
		}

		// Act
		err := startAndRunTool(client, rs, filepath.Join(t.TempDir(), "proxy_abc.jsonl"))

		// Assert
		require.NoError(t, err)
		args := get()
		assert.True(t, hasArgWithPrefix(args, "--network=agentic-run-"))
		assert.Contains(t, args, "--env=NO_PROXY=localhost,127.0.0.1,agentic-docker")
	})

	t.Run("maps the spec's mounts, entrypoint, image and tool args", func(t *testing.T) {
		// Arrange
		rs := sandbox.RunSpec{
			Image:          "agentic-claude",
			ToolHome:       "/home/.agentic",
			ContainerHome:  "/home/user",
			Volumes:        []string{"/host:/container", "$TOOL_HOME/data:/data"},
			EphemeralDirs:  []sandbox.EphemeralDir{{Path: "/tmp", Size: "1g"}, {Path: "$CONTAINER_HOME/.cache", Size: "1g"}},
			SkipEntrypoint: true,
			ToolArgs:       []string{"--resume"},
		}

		// Act
		err := startAndRunTool(client, rs, "")

		// Assert
		require.NoError(t, err)
		args := get()
		assert.Contains(t, args, "--volume=/host:/container")
		assert.Contains(t, args, "--volume=/home/.agentic/data:/data")
		assert.Contains(t, args, "--tmpfs=/tmp:exec,size=1g")
		assert.Contains(t, args, "--tmpfs=/home/user/.cache:exec,size=1g")
		assert.Equal(t, "", argAfter(args, "--entrypoint"))
		assert.Equal(t, []string{"agentic-claude", "--resume"}, args[len(args)-2:])
	})
}

func TestClient_StopSidecars(t *testing.T) {
	client := newTestClient()

	t.Run("tears its run down", func(t *testing.T) {
		// Arrange
		stubDindReadyTimeout(t, client)
		toolHome := t.TempDir()
		daemon := &fakeDaemon{}
		stubDaemon(t, client, daemon)
		rs := sandbox.RunSpec{
			RunID:    testRunID,
			Image:    "agentic-claude",
			User:     &testUser,
			ToolHome: toolHome,
			Dind:     sandbox.DindSpec{Enabled: true},
		}
		sidecars, err := client.StartSidecars(rs, "")
		require.NoError(t, err)
		daemon.containers = []ownedResource{owned(dindContainerName(testRunID), sidecars.run.container, 0)}
		daemon.networks = []ownedResource{owned(runNetworkName(testRunID), sidecars.run.container, 0)}

		// Act
		client.StopSidecars(sidecars)

		// Assert
		assert.Empty(t, daemon.names())
		assert.NoDirExists(t, dindRunDir(toolHome, testRunID))
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
