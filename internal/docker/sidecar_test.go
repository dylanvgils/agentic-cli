package docker

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ownerLabels(t *testing.T) {
	// Arrange
	rs := RunSpec{container: "agentic-claude-abc"}

	// Act
	labels := ownerLabels(rs)

	// Assert
	require.Len(t, labels, 2)
	assert.Equal(t, "--label=agentic.owner=agentic-claude-abc", labels[0])
	assert.True(t, hasArgWithPrefix(labels[1:], "--label=agentic.started="))
}

func Test_setupSidecars(t *testing.T) {
	t.Run("sweeps orphaned sidecars first", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t)
		rs := RunSpec{Image: "agentic-claude"}

		// Act
		_, _, err := setupSidecars(&rs)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, findCall(calls(), "ps"))
	})

	t.Run("dry run does not sweep", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t)
		rs := RunSpec{Image: "agentic-claude", DryRun: true}

		// Act
		_, _, err := setupSidecars(&rs)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, calls())
	})

	t.Run("stops the proxy when dind fails to start", func(t *testing.T) {
		// Arrange
		stubHostUserGroup(t, "1000:1000")
		stubDindReadyTimeout(t)
		calls := stubDockerRunCapture(t, "network inspect", "exec")
		rs := RunSpec{
			Image:    "agentic-claude",
			ToolHome: t.TempDir(),
			Proxy:    ProxySpec{Mode: ProxyEnforce, Image: "default-proxy", LogDir: t.TempDir()},
			Dind:     DindSpec{Enabled: true, Image: "dind"},
		}

		// Act
		_, _, err := setupSidecars(&rs)

		// Assert
		require.Error(t, err)
		proxyRemoved := false
		for _, c := range calls() {
			if c.args[0] == "rm" && hasArgWithPrefix(c.args, "agentic-proxy-") {
				proxyRemoved = true
			}
		}
		assert.True(t, proxyRemoved, "the proxy must not outlive a failed run")
	})
}

func Test_sweepOrphanedSidecars(t *testing.T) {
	stubSidecarOrphanGrace(t, 5*time.Minute)

	t.Run("removes an old sidecar and network whose owner is gone", func(t *testing.T) {
		// Arrange
		get := stubOwnedResources(t,
			ownedRow("agentic-docker-abc", "agentic-claude-gone", time.Hour),
			ownedRow("agentic-dind-abc", "agentic-claude-gone", time.Hour),
		)

		// Act
		err := sweepOrphanedSidecars("")

		// Assert
		require.NoError(t, err)
		calls := get()
		assert.Equal(t, []string{"rm", "--force", "--volumes", "agentic-docker-abc"}, findCall(calls, "rm"))
		assert.Equal(t, []string{"network", "rm", "agentic-dind-abc"}, findCall(calls, "network", "rm"))
	})

	t.Run("keeps a sidecar younger than the grace period", func(t *testing.T) {
		// Arrange
		get := stubOwnedResources(t,
			ownedRow("agentic-proxy-abc", "agentic-claude-starting", time.Minute),
			ownedRow("agentic-proxy-abc", "agentic-claude-starting", time.Minute),
		)

		// Act
		err := sweepOrphanedSidecars("")

		// Assert
		require.NoError(t, err)
		calls := get()
		assert.Nil(t, findCall(calls, "rm"))
		assert.Nil(t, findCall(calls, "network", "rm"))
	})

	t.Run("keeps a sidecar whose owner still exists", func(t *testing.T) {
		// Arrange
		containers := ownedRow("agentic-claude-abc", "", 0) + ownedRow("agentic-proxy-abc", "agentic-claude-abc", time.Hour)
		get := stubOwnedResources(t, containers, ownedRow("agentic-proxy-abc", "agentic-claude-abc", time.Hour))

		// Act
		err := sweepOrphanedSidecars("")

		// Assert
		require.NoError(t, err)
		calls := get()
		assert.Nil(t, findCall(calls, "rm"))
		assert.Nil(t, findCall(calls, "network", "rm"))
	})

	t.Run("ignores resources without an owner or start time", func(t *testing.T) {
		// Arrange
		containers := "agentic-proxy-old\t\t\n" + "agentic-docker-old\tagentic-claude-gone\t\n"
		get := stubOwnedResources(t, containers, "")

		// Act
		err := sweepOrphanedSidecars("")

		// Assert
		require.NoError(t, err)
		assert.Nil(t, findCall(get(), "rm"))
	})

	t.Run("a failed removal doesn't stop the rest", func(t *testing.T) {
		// Arrange
		containers := ownedRow("agentic-docker-a", "agentic-claude-gone", time.Hour) + ownedRow("agentic-docker-b", "agentic-claude-gone", time.Hour)
		var removed []string
		stubDockerRun(t, func(args ...string) (string, error) {
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
		err := sweepOrphanedSidecars("")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"agentic-docker-a", "agentic-docker-b"}, removed)
	})

	t.Run("list error propagates", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", errors.New("stub: daemon down"))

		// Act
		err := sweepOrphanedSidecars("")

		// Assert
		assert.ErrorContains(t, err, "daemon down")
	})

	t.Run("sweeps run dirs left behind by crashed runs", func(t *testing.T) {
		// Arrange
		stubRunDirGrace(t, time.Minute)
		stubOwnedResources(t, "", "")
		toolHome := t.TempDir()
		staleDind := makeRunDir(t, toolHome, dindDirName, "0123456789ab", time.Hour)
		staleProxy := makeRunDir(t, toolHome, proxyDirName, "0123456789ab", time.Hour)

		// Act
		err := sweepOrphanedSidecars(toolHome)

		// Assert
		require.NoError(t, err)
		assert.NoDirExists(t, staleDind)
		assert.NoDirExists(t, staleProxy)
	})
}

func Test_sweepRunDirs(t *testing.T) {
	stubRunDirGrace(t, time.Minute)

	t.Run("removes a stale dir whose sidecar is gone", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", nil)
		toolHome := t.TempDir()
		dir := makeRunDir(t, toolHome, dindDirName, "0123456789ab", time.Hour)

		// Act
		err := sweepRunDirs(toolHome, dindDirName, dindHostAlias)

		// Assert
		require.NoError(t, err)
		assert.NoDirExists(t, dir)
	})

	t.Run("keeps a dir whose sidecar still exists", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "agentic-docker-0123456789ab\n", nil)
		toolHome := t.TempDir()
		dir := makeRunDir(t, toolHome, dindDirName, "0123456789ab", time.Hour)

		// Act
		err := sweepRunDirs(toolHome, dindDirName, dindHostAlias)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, dir)
	})

	t.Run("keeps a dir younger than the grace period", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", nil)
		toolHome := t.TempDir()
		dir := makeRunDir(t, toolHome, dindDirName, "0123456789ab", 0)

		// Act
		err := sweepRunDirs(toolHome, dindDirName, dindHostAlias)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, dir, "a concurrent run may not have started its sidecar yet")
	})

	t.Run("ignores entries that are not run ids", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", nil)
		toolHome := t.TempDir()
		dir := makeRunDir(t, toolHome, dindDirName, "not-a-run-id", time.Hour)

		// Act
		err := sweepRunDirs(toolHome, dindDirName, dindHostAlias)

		// Assert
		require.NoError(t, err)
		assert.DirExists(t, dir)
	})

	t.Run("missing run dir base is a no-op without calling docker", func(t *testing.T) {
		// Arrange
		called := false
		stubDockerRun(t, func(...string) (string, error) {
			called = true
			return "", nil
		})

		// Act
		err := sweepRunDirs(t.TempDir(), dindDirName, dindHostAlias)

		// Assert
		require.NoError(t, err)
		assert.False(t, called)
	})

	t.Run("docker error propagates and keeps dirs", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "", fmt.Errorf("daemon down"))
		toolHome := t.TempDir()
		dir := makeRunDir(t, toolHome, dindDirName, "0123456789ab", time.Hour)

		// Act
		err := sweepRunDirs(toolHome, dindDirName, dindHostAlias)

		// Assert
		require.Error(t, err)
		assert.DirExists(t, dir, "without the live list a running sidecar's certs could be deleted")
	})
}
