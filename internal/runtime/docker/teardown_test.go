package docker

import (
	"errors"
	"os"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Teardown(t *testing.T) {
	client := newTestClient()
	tool := "agentic-claude-" + string(testRunID)
	proxyName := proxyContainerName(testRunID)
	dindName := dindContainerName(testRunID)
	network := runNetworkName(testRunID)

	t.Run("removes the tool, then its sidecars, network and files", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		makeDindRunDir(t, toolHome, string(testRunID), 0)
		require.NoError(t, os.WriteFile(tokenFilePath(testRunID), []byte("token"), 0o600))
		daemon := &fakeDaemon{
			containers: []ownedResource{
				owned(proxyName, tool, testRunID, 0),
				owned(tool, pidOwner(sandbox.RuntimeDocker, os.Getpid()), testRunID, 0),
				owned(dindName, tool, testRunID, 0),
				owned("agentic-claude-"+string(otherRunID), "", otherRunID, 0),
			},
			networks: []ownedResource{
				owned(network, tool, testRunID, 0),
			},
		}
		stubDaemon(t, client, daemon)

		// Act
		err := client.Teardown(toolHome, testRunID)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{
			tool,
			proxyName,
			dindName,
			network,
		}, daemon.removed())
		assert.Equal(t, []string{"agentic-claude-" + string(otherRunID)}, daemon.names(), "other runs are kept")
		assert.NoDirExists(t, dindRunDir(toolHome, testRunID))
		assert.NoFileExists(t, tokenFilePath(testRunID))
	})

	t.Run("removes what a partly created run has", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		makeDindRunDir(t, toolHome, string(testRunID), 0)
		daemon := &fakeDaemon{networks: []ownedResource{owned(network, tool, testRunID, 0)}}
		stubDaemon(t, client, daemon)

		// Act
		err := client.Teardown(toolHome, testRunID)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, daemon.names())
		assert.NoDirExists(t, dindRunDir(toolHome, testRunID))
	})

	t.Run("a run already torn down is a no-op", func(t *testing.T) {
		// Arrange
		toolHome := t.TempDir()
		daemon := &fakeDaemon{containers: []ownedResource{owned(tool, "", testRunID, 0)}}
		stubDaemon(t, client, daemon)

		// Act
		first := client.Teardown(toolHome, testRunID)
		second := client.Teardown(toolHome, testRunID)

		// Assert
		require.NoError(t, first)
		require.NoError(t, second)
		assert.Empty(t, daemon.names())
	})

	t.Run("a container gone between listing and removal counts as removed", func(t *testing.T) {
		// Arrange
		stubDockerRun(t, client, func(args ...string) (string, error) {
			switch args[0] {
			case "ps":
				return tool + "\t\t\t" + string(testRunID) + "\n", nil
			case "rm":
				return "", errors.New("Error response from daemon: removal of container " + tool + " is already in progress")
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
			containers: []ownedResource{
				owned(tool, "", testRunID, 0),
			},
			networks: []ownedResource{
				owned(network, tool, testRunID, 0),
			},
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
