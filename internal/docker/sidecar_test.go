package docker

import (
	"errors"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ownerLabels(t *testing.T) {
	// Arrange
	rs := toolRun{container: "agentic-claude-abc"}

	// Act
	labels := ownerLabels(rs)

	// Assert
	require.Len(t, labels, 2)
	assert.Equal(t, "--label=agentic.owner=agentic-claude-abc", labels[0])
	assert.True(t, hasArgWithPrefix(labels[1:], "--label=agentic.started="))
}

func Test_setupSidecars(t *testing.T) {
	client := newTestClient()

	t.Run("sweeps orphaned sidecars first", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client)
		rs := toolRun{RunSpec: sandbox.RunSpec{Image: "agentic-claude"}}

		// Act
		_, _, err := client.setupSidecars(rs)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, findCall(calls(), "ps"))
	})

	t.Run("creates the run network after the sweep and removes it in cleanup", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client)
		rs := toolRun{RunSpec: sandbox.RunSpec{Image: "agentic-claude"}, network: "agentic-run-abc"}

		// Act
		_, cleanup, err := client.setupSidecars(rs)
		cleanup()

		// Assert
		require.NoError(t, err)
		got := calls()
		assert.Equal(t, "ps", got[0].args[0])
		createArgs := findCall(got, "network", "create")
		assert.Equal(t, "agentic-run-abc", createArgs[len(createArgs)-1])
		assert.Equal(t, []string{"network", "rm", "agentic-run-abc"}, got[len(got)-1].args)
	})

	t.Run("removes the run network when the proxy fails to start", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client, "network inspect", "create")
		rs := toolRun{RunSpec: sandbox.RunSpec{
			Image: "agentic-claude",
			Proxy: sandbox.ProxySpec{Mode: sandbox.ProxyEnforce, LogDir: t.TempDir()},
		}, network: "agentic-run-abc"}

		// Act
		_, _, err := client.setupSidecars(rs)

		// Assert
		require.ErrorContains(t, err, "create proxy")
		got := calls()
		assert.Equal(t, []string{"network", "rm", "agentic-run-abc"}, got[len(got)-1].args)
	})

	t.Run("a failed network create starts no sidecar", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client, "network create")
		rs := toolRun{RunSpec: sandbox.RunSpec{
			Image: "agentic-claude",
			Proxy: sandbox.ProxySpec{Mode: sandbox.ProxyEnforce, LogDir: t.TempDir()},
		}, network: "agentic-run-abc"}

		// Act
		_, _, err := client.setupSidecars(rs)

		// Assert
		require.ErrorContains(t, err, "create run network")
		assert.Nil(t, findCall(calls(), "create"))
	})

	t.Run("dry run makes no docker calls", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client)
		rs := toolRun{RunSpec: sandbox.RunSpec{Image: "agentic-claude", DryRun: true}}

		// Act
		_, _, err := client.setupSidecars(rs)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, calls())
	})

	t.Run("stops the proxy, then removes the run network, when dind fails to start", func(t *testing.T) {
		// Arrange
		stubHostUserGroup(t, "1000:1000")
		stubDindReadyTimeout(t)
		calls := stubDockerRunCapture(t, client, "network inspect", "exec")
		rs := toolRun{RunSpec: sandbox.RunSpec{
			Image:    "agentic-claude",
			ToolHome: t.TempDir(),
			Proxy:    sandbox.ProxySpec{Mode: sandbox.ProxyEnforce, LogDir: t.TempDir()},
			Dind:     sandbox.DindSpec{Enabled: true},
		}, network: "agentic-run-abc"}

		// Act
		_, _, err := client.setupSidecars(rs)

		// Assert
		require.Error(t, err)
		got := calls()
		proxyRemoved := false
		for _, c := range got {
			if c.args[0] == "rm" && hasArgWithPrefix(c.args, "agentic-proxy-") {
				proxyRemoved = true
			}
		}
		assert.True(t, proxyRemoved, "the proxy must not outlive a failed run")
		assert.Equal(t, []string{"network", "rm", "agentic-run-abc"}, got[len(got)-1].args)
	})
}

func Test_sweepOrphanedSidecars(t *testing.T) {
	client := newTestClient()

	stubSidecarOrphanGrace(t, 5*time.Minute)

	t.Run("removes an old sidecar and network whose owner is gone", func(t *testing.T) {
		// Arrange
		get := stubOwnedResources(t, client,
			ownedRow("agentic-docker-abc", "agentic-claude-gone", time.Hour),
			ownedRow("agentic-run-abc", "agentic-claude-gone", time.Hour),
		)

		// Act
		err := client.sweepOrphanedSidecars("")

		// Assert
		require.NoError(t, err)
		calls := get()
		assert.Equal(t, []string{"rm", "--force", "--volumes", "agentic-docker-abc"}, findCall(calls, "rm"))
		assert.Equal(t, []string{"network", "rm", "agentic-run-abc"}, findCall(calls, "network", "rm"))
	})

	t.Run("keeps a sidecar younger than the grace period", func(t *testing.T) {
		// Arrange
		get := stubOwnedResources(t, client,
			ownedRow("agentic-proxy-abc", "agentic-claude-starting", time.Minute),
			ownedRow("agentic-run-abc", "agentic-claude-starting", time.Minute),
		)

		// Act
		err := client.sweepOrphanedSidecars("")

		// Assert
		require.NoError(t, err)
		calls := get()
		assert.Nil(t, findCall(calls, "rm"))
		assert.Nil(t, findCall(calls, "network", "rm"))
	})

	t.Run("keeps a sidecar whose owner still exists", func(t *testing.T) {
		// Arrange
		containers := ownedRow("agentic-claude-abc", "", 0) + ownedRow("agentic-proxy-abc", "agentic-claude-abc", time.Hour)
		get := stubOwnedResources(t, client, containers, ownedRow("agentic-run-abc", "agentic-claude-abc", time.Hour))

		// Act
		err := client.sweepOrphanedSidecars("")

		// Assert
		require.NoError(t, err)
		calls := get()
		assert.Nil(t, findCall(calls, "rm"))
		assert.Nil(t, findCall(calls, "network", "rm"))
	})

	t.Run("ignores resources without an owner or start time", func(t *testing.T) {
		// Arrange
		containers := "agentic-proxy-old\t\t\n" + "agentic-docker-old\tagentic-claude-gone\t\n"
		get := stubOwnedResources(t, client, containers, "")

		// Act
		err := client.sweepOrphanedSidecars("")

		// Assert
		require.NoError(t, err)
		assert.Nil(t, findCall(get(), "rm"))
	})

	t.Run("a failed removal doesn't stop the rest", func(t *testing.T) {
		// Arrange
		containers := ownedRow("agentic-docker-a", "agentic-claude-gone", time.Hour) + ownedRow("agentic-docker-b", "agentic-claude-gone", time.Hour)
		var removed []string
		stubDockerRun(t, client, func(args ...string) (string, error) {
			switch args[0] {
			case "ps":
				return containers, nil
			case "rm":
				removed = append(removed, args[len(args)-1])
				return "", errors.New("stub: rm failed")
			}
			return "", nil
		})

		// Act
		err := client.sweepOrphanedSidecars("")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"agentic-docker-a", "agentic-docker-b"}, removed)
	})

	t.Run("list error propagates", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", errors.New("stub: daemon down"))

		// Act
		err := client.sweepOrphanedSidecars("")

		// Assert
		assert.ErrorContains(t, err, "daemon down")
	})

	t.Run("sweeps run dirs left behind by crashed runs", func(t *testing.T) {
		// Arrange
		stubDindRunDirGrace(t, time.Minute)
		stubOwnedResources(t, client, "", "")
		toolHome := t.TempDir()
		stale := makeDindRunDir(t, toolHome, "0123456789ab", time.Hour)

		// Act
		err := client.sweepOrphanedSidecars(toolHome)

		// Assert
		require.NoError(t, err)
		assert.NoDirExists(t, stale)
	})
}
