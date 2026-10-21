package docker

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
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
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Image: "agentic-claude"}}

		// Act
		_, _, err := client.setupSidecars(rs)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, findCall(calls(), "ps"))
	})

	t.Run("creates the run network after the sweep and removes it in cleanup", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client)
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Image: "agentic-claude"}, network: "agentic-run-abc"}

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
		calls := stubProxyDocker(t, client, "true", "create")
		rs := toolRun{RunSpec: sandbox.RunSpec{
			User:  &testUser,
			Image: "agentic-claude",
			Proxy: sandbox.ProxySpec{Mode: sandbox.ProxyEnforce, LogFile: filepath.Join(t.TempDir(), "proxy_abc.jsonl")},
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
			User:  &testUser,
			Image: "agentic-claude",
			Proxy: sandbox.ProxySpec{Mode: sandbox.ProxyEnforce, LogFile: filepath.Join(t.TempDir(), "proxy_abc.jsonl")},
		}, network: "agentic-run-abc"}

		// Act
		_, _, err := client.setupSidecars(rs)

		// Assert
		require.ErrorContains(t, err, "create run network")
		assert.Nil(t, findCall(calls(), "create"))
	})

	t.Run("dry run prints the run network and makes no docker calls", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client)
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Image: "agentic-claude", DryRun: true, Proxy: sandbox.ProxySpec{Mode: sandbox.ProxyEnforce}}, network: "agentic-run-abc"}

		// Act
		var err error
		out := captureStdout(t, func() { _, _, err = client.setupSidecars(rs) })

		// Assert
		require.NoError(t, err)
		network, _, _ := strings.Cut(out, "docker create")
		assert.True(t, strings.HasPrefix(network, "docker network"), "the network comes first, as the sidecars join it")
		assert.Contains(t, network, "--internal")
		assert.Contains(t, network, "agentic-run-abc")
		assert.Empty(t, calls())
	})

	t.Run("stops the proxy, then removes the run network, when dind fails to start", func(t *testing.T) {
		// Arrange
		stubDindReadyTimeout(t, client)
		calls := stubProxyDocker(t, client, "true", "exec")
		rs := toolRun{RunSpec: sandbox.RunSpec{
			User:     &testUser,
			Image:    "agentic-claude",
			ToolHome: t.TempDir(),
			Proxy:    sandbox.ProxySpec{Mode: sandbox.ProxyEnforce, LogFile: filepath.Join(t.TempDir(), "proxy_abc.jsonl")},
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
		stubDindRunDirGrace(t, client, time.Minute)
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

func Test_ownedResource_inUse(t *testing.T) {
	existing := map[string]bool{"agentic-claude-abc": true}

	t.Run("owner still exists", func(t *testing.T) {
		// Arrange
		r := ownedResource{name: "agentic-proxy-abc", owner: "agentic-claude-abc", started: time.Now().Add(-time.Hour)}

		// Act
		inUse := r.inUse(existing, 5*time.Minute)

		// Assert
		assert.True(t, inUse)
	})

	t.Run("owner not started yet", func(t *testing.T) {
		// Arrange
		r := ownedResource{name: "agentic-proxy-abc", owner: "agentic-claude-starting", started: time.Now()}

		// Act
		inUse := r.inUse(existing, 5*time.Minute)

		// Assert
		assert.True(t, inUse)
	})

	t.Run("owner gone", func(t *testing.T) {
		// Arrange
		r := ownedResource{name: "agentic-proxy-abc", owner: "agentic-claude-gone", started: time.Now().Add(-time.Hour)}

		// Act
		inUse := r.inUse(existing, 5*time.Minute)

		// Assert
		assert.False(t, inUse)
	})

	t.Run("no owner", func(t *testing.T) {
		// Arrange
		r := ownedResource{name: "agentic-proxy-old"}

		// Act
		inUse := r.inUse(existing, 5*time.Minute)

		// Assert
		assert.False(t, inUse)
	})
}

func Test_removeUnused(t *testing.T) {
	client := newTestClient()
	listArgs := []string{"ps", "--all", "--format=owned"}
	rmArgs := []string{"rm", "--force"}

	t.Run("removes the unused resources in one call", func(t *testing.T) {
		// Arrange
		owned := ownedRow("agentic-proxy-a", "agentic-claude-gone", time.Hour) + ownedRow("agentic-proxy-b", "agentic-claude-gone", time.Hour)
		get := stubUnusedSweep(t, client, "", owned)

		// Act
		err := client.removeUnused(listArgs, rmArgs)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"rm", "--force", "agentic-proxy-a", "agentic-proxy-b"}, findCall(get(), "rm"))
	})

	t.Run("nothing unused makes no rm call", func(t *testing.T) {
		// Arrange
		get := stubUnusedSweep(t, client, "agentic-claude-live\n", ownedRow("agentic-proxy-live", "agentic-claude-live", time.Hour))

		// Act
		err := client.removeUnused(listArgs, rmArgs)

		// Assert
		require.NoError(t, err)
		assert.Nil(t, findCall(get(), "rm"))
	})

	t.Run("list error propagates", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", errors.New("stub: daemon down"))

		// Act
		err := client.removeUnused(listArgs, rmArgs)

		// Assert
		assert.ErrorContains(t, err, "daemon down")
	})
}

func Test_unusedNames(t *testing.T) {
	// Arrange
	started := time.Now().Add(-time.Hour)
	resources := []ownedResource{
		{name: "agentic-proxy-live", owner: "agentic-claude-live", started: started},
		{name: "agentic-proxy-gone", owner: "agentic-claude-gone", started: started},
		{name: "agentic-proxy-old"},
	}
	existing := map[string]bool{"agentic-claude-live": true}

	// Act
	names := unusedNames(resources, existing, 5*time.Minute)

	// Assert
	assert.Equal(t, []string{"agentic-proxy-gone", "agentic-proxy-old"}, names)
}
