package docker

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Teardown(t *testing.T) {
	client := newTestClient()
	tool := "agentic-claude-" + string(testRunID)
	proxyName := proxyContainerName(testRunID)
	dindName := dindContainerName(testRunID)
	network := runNetworkName(testRunID)
	t.Setenv("TMPDIR", t.TempDir())

	t.Run("removes the sidecars, then the network and files", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		makeDindRunDir(t, toolHome, string(testRunID), 0)
		require.NoError(t, os.WriteFile(tokenFilePath(testRunID), []byte("token"), 0o600))
		daemon := &fakeDaemon{
			containers: []ownedResource{
				owned(proxyName, tool, 0),
				owned(dindName, tool, 0),
				owned("agentic-proxy-ba9876543210", "agentic-claude-ba9876543210", 0),
			},
			networks: []ownedResource{
				owned(network, tool, 0),
			},
		}
		stubDaemon(t, client, daemon)

		// Act
		err := client.Teardown(toolHome, testRunID)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{
			dindName,
			proxyName,
			network,
		}, daemon.removed())
		assert.Equal(t, []string{"agentic-proxy-ba9876543210"}, daemon.names(), "other runs are kept")
		assert.NoDirExists(t, dindRunDir(toolHome, testRunID))
		assert.NoFileExists(t, tokenFilePath(testRunID))
	})

	t.Run("a run already torn down is a no-op", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		daemon := &fakeDaemon{networks: []ownedResource{owned(network, tool, 0)}}
		stubDaemon(t, client, daemon)

		// Act
		first := client.Teardown(toolHome, testRunID)
		second := client.Teardown(toolHome, testRunID)

		// Assert
		require.NoError(t, first)
		require.NoError(t, second)
		assert.Empty(t, daemon.names())
	})

	t.Run("a removal already in progress counts as removed", func(t *testing.T) {
		// Arrange
		stubDockerRun(t, client, func(args ...string) (string, error) {
			if args[0] == "rm" {
				return "", errors.New("Error response from daemon: removal of container " + proxyName + " is already in progress")
			}
			return "", errors.New("Error response from daemon: network " + network + " not found")
		})

		// Act
		err := client.Teardown("", testRunID)

		// Assert
		require.NoError(t, err)
	})

	t.Run("joins errors and still runs every step", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		makeDindRunDir(t, toolHome, string(testRunID), 0)
		daemon := &fakeDaemon{
			failing: [][]string{
				{"rm"},
				{"network", "rm"},
			},
		}
		stubDaemon(t, client, daemon)

		// Act
		err := client.Teardown(toolHome, testRunID)

		// Assert
		require.ErrorContains(t, err, "stub: rm failed")
		require.ErrorContains(t, err, "stub: network rm failed")
		assert.NoDirExists(t, dindRunDir(toolHome, testRunID))
	})

	t.Run("refuses an invalid run id before touching Docker", func(t *testing.T) {
		// Arrange
		daemon := &fakeDaemon{}
		stubDaemon(t, client, daemon)

		// Act
		err := client.Teardown(t.TempDir(), "../x")

		// Assert
		require.EqualError(t, err, `invalid run id "../x"`)
		assert.Empty(t, daemon.calls)
	})
}
