package docker

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_setupSidecars(t *testing.T) {
	client := newTestClient()
	network := runNetworkName(testRunID)

	t.Run("sweeps orphans, then creates the run network", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client)
		rs := toolRun{
			RunSpec: sandbox.RunSpec{
				RunID: testRunID,
				User:  &testUser,
				Image: "agentic-claude",
			},
			network: network,
		}

		// Act
		_, guard, err := client.setupSidecars(rs)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, guard)
		got := calls()
		assert.Equal(t, "ps", got[0].args[0])
		createArgs := findCall(got, "network", "create")
		assert.Equal(t, network, createArgs[len(createArgs)-1])
		assert.Nil(t, findCall(got, "network", "rm"))
	})

	t.Run("tears the run down when the proxy fails to start", func(t *testing.T) {
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
		assert.NotNil(t, findCall(got, runContainersArgs(testRunID)...))
		assert.Equal(t, []string{"network", "rm", network}, got[len(got)-1].args)
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

	t.Run("tears the proxy, network and run dir down when dind fails to start", func(t *testing.T) {
		// Arrange
		stubDindReadyTimeout(t, client)
		toolHome := t.TempDir()
		daemon := &fakeDaemon{failing: [][]string{{"exec"}}}
		stubDaemon(t, client, daemon)
		rs := newProxyRun(t, sandbox.RunSpec{
			Image:    "agentic-claude",
			ToolHome: toolHome,
			Dind:     sandbox.DindSpec{Enabled: true},
		})
		rs.network = network
		tool := "agentic-claude-" + string(testRunID)
		daemon.containers = []ownedResource{
			owned(dindContainerName(testRunID), tool, testRunID, 0),
		}
		daemon.networks = []ownedResource{
			owned(network, tool, testRunID, 0),
		}

		// Act
		_, _, err := client.setupSidecars(rs)

		// Assert
		require.Error(t, err)
		assert.Empty(t, daemon.names())
		assert.NoDirExists(t, dindRunDir(toolHome, testRunID))
	})
}
