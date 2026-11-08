package docker

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams/iostreamstest"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ownerLabels(t *testing.T) {
	// Arrange
	// Act
	labels := ownerLabels("agentic-claude-abc")

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
		rs := newProxyRun(t, sandbox.RunSpec{
			Image:  "agentic-claude",
			Egress: sandbox.EgressPolicy{Mode: sandbox.EgressEnforce},
		})

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
		rs := newProxyRun(t, sandbox.RunSpec{
			Image:  "agentic-claude",
			Egress: sandbox.EgressPolicy{Mode: sandbox.EgressEnforce},
		})

		// Act
		_, _, err := client.setupSidecars(rs)

		// Assert
		require.ErrorContains(t, err, "create run network")
		assert.Nil(t, findCall(calls(), "create"))
	})

	t.Run("dry run prints the run network and makes no docker calls", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client)
		rs := toolRun{RunSpec: sandbox.RunSpec{User: &testUser, Image: "agentic-claude", DryRun: true, Egress: sandbox.EgressPolicy{Mode: sandbox.EgressEnforce}}, network: "agentic-run-abc"}

		// Act
		var err error
		out := iostreamstest.CaptureOut(t, func() { _, _, err = client.setupSidecars(rs) })

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
		rs := newProxyRun(t, sandbox.RunSpec{
			Image:    "agentic-claude",
			ToolHome: t.TempDir(),
			Egress:   sandbox.EgressPolicy{Mode: sandbox.EgressEnforce},
			Dind:     sandbox.DindSpec{Enabled: true},
		})

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

	t.Run("keeps sidecars and networks still in use", func(t *testing.T) {
		// Arrange - one owner still exists, the other is younger than the grace period
		containers := ownedRow("agentic-claude-abc", "", 0) +
			ownedRow("agentic-proxy-abc", "agentic-claude-abc", time.Hour) +
			ownedRow("agentic-proxy-def", "agentic-claude-starting", time.Minute)
		networks := ownedRow("agentic-run-abc", "agentic-claude-abc", time.Hour) + ownedRow("agentic-run-def", "agentic-claude-starting", time.Minute)
		get := stubOwnedResources(t, client, containers, networks)

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
	old := time.Now().Add(-time.Hour)
	gonePID := exitedPID(t)

	for _, tc := range []struct {
		name    string
		owner   string
		started time.Time
		want    bool
	}{
		{name: "owner still exists", owner: "agentic-claude-abc", started: old, want: true},
		{name: "owner not started yet", owner: "agentic-claude-starting", started: time.Now(), want: true},
		{name: "owner gone", owner: "agentic-claude-gone", started: old},
		{name: "no owner"},
		{name: "host process owner still runs", owner: pidOwner(sandbox.RuntimeMicrosandbox, os.Getpid()), started: old, want: true},
		{name: "host process owner gone", owner: pidOwner(sandbox.RuntimeMicrosandbox, gonePID), started: old},
		{name: "host process owner gone but within grace", owner: pidOwner(sandbox.RuntimeMicrosandbox, gonePID), started: time.Now(), want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			r := ownedResource{name: "agentic-proxy-abc", owner: tc.owner, started: tc.started}

			// Act
			inUse := r.inUse(existing, 5*time.Minute)

			// Assert
			assert.Equal(t, tc.want, inUse)
		})
	}
}

func Test_ownerPID(t *testing.T) {
	t.Run("parses a pid owner", func(t *testing.T) {
		// Act
		pid, ok := ownerPID(pidOwner(sandbox.RuntimeMicrosandbox, 4242))

		// Assert
		assert.True(t, ok)
		assert.Equal(t, 4242, pid)
	})

	t.Run("refuses other owners", func(t *testing.T) {
		for _, owner := range []string{"agentic-claude-abc", "microsandbox:pid:", "microsandbox:pid:abc", "microsandbox:pid:0", "microsandbox:pid:-1"} {
			// Act
			_, ok := ownerPID(owner)

			// Assert
			assert.False(t, ok, owner)
		}
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
		removed, err := client.removeUnused(listArgs, rmArgs)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"agentic-proxy-a", "agentic-proxy-b"}, removed)
		assert.Equal(t, []string{"rm", "--force", "agentic-proxy-a", "agentic-proxy-b"}, findCall(get(), "rm"))
	})

	t.Run("nothing unused makes no rm call", func(t *testing.T) {
		// Arrange
		get := stubUnusedSweep(t, client, "agentic-claude-live\n", ownedRow("agentic-proxy-live", "agentic-claude-live", time.Hour))

		// Act
		removed, err := client.removeUnused(listArgs, rmArgs)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, removed)
		assert.Nil(t, findCall(get(), "rm"))
	})

	t.Run("list error propagates", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", errors.New("stub: daemon down"))

		// Act
		_, err := client.removeUnused(listArgs, rmArgs)

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
