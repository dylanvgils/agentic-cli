package docker

import (
	"fmt"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveNetwork(t *testing.T) {
	client := newTestClient()

	t.Run("network does not exist returns nil", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, client, "network inspect")

		// Act
		err := client.RemoveNetwork()

		// Assert
		require.NoError(t, err)
	})

	t.Run("wrong label returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "other-project\n", nil)

		// Act
		err := client.RemoveNetwork()

		// Assert
		assert.ErrorContains(t, err, "not an agentic-managed network")
	})

	t.Run("agentic-managed network calls rm", func(t *testing.T) {
		// Arrange
		var calls []dockerCall
		stubDockerRun(t, client, func(args ...string) (string, error) {
			calls = append(calls, dockerCall{args: args})
			if args[0] == "network" && args[1] == "inspect" {
				return "agentic-cli\n", nil
			}
			return "", nil
		})

		// Act
		err := client.RemoveNetwork()

		// Assert
		require.NoError(t, err)
		require.Len(t, calls, 2)
		assert.Equal(t, "inspect", calls[0].args[1])
		assert.Equal(t, []string{"network", "rm", NetworkName}, calls[1].args)
	})

	t.Run("rm fails propagates error", func(t *testing.T) {
		// Arrange
		stubDockerRun(t, client, func(args ...string) (string, error) {
			if args[0] == "network" && args[1] == "inspect" {
				return "agentic-cli\n", nil
			}
			return "", fmt.Errorf("stub: network rm failed")
		})

		// Act
		err := client.RemoveNetwork()

		// Assert
		assert.ErrorContains(t, err, "network rm failed")
	})
}

func TestSweepRunNetworks(t *testing.T) {
	client := newTestClient()

	t.Run("removes every owned network", func(t *testing.T) {
		// Arrange
		var calls [][]string
		stubDockerRun(t, client, func(args ...string) (string, error) {
			calls = append(calls, args)
			if args[1] == "ls" {
				return "id1\nid2\n", nil
			}
			return "", nil
		})

		// Act
		err := client.SweepRunNetworks()

		// Assert
		require.NoError(t, err)
		require.Len(t, calls, 2)
		assert.Contains(t, calls[0], "--filter=label=project=agentic-cli")
		assert.Contains(t, calls[0], "--filter=label=agentic.owner")
		assert.Equal(t, []string{"network", "rm", "id1", "id2"}, calls[1])
	})

	t.Run("none left makes no rm call", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client)

		// Act
		err := client.SweepRunNetworks()

		// Assert
		require.NoError(t, err)
		assert.Nil(t, findCall(get(), "network", "rm"))
	})
}

func Test_ensureEgressNetwork(t *testing.T) {
	client := newTestClient()

	t.Run("existing network skips create", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client)

		// Act
		err := client.ensureEgressNetwork()

		// Assert
		require.NoError(t, err)
		calls := get()
		require.Len(t, calls, 1)
		assert.Equal(t, []string{"network", "inspect", NetworkName}, calls[0].args)
	})

	t.Run("missing network creates with label", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client, "network inspect")

		// Act
		err := client.ensureEgressNetwork()

		// Assert
		require.NoError(t, err)
		calls := get()
		require.Len(t, calls, 2)
		assert.Equal(t, []string{"network", "inspect", NetworkName}, calls[0].args)
		assert.Equal(t, []string{"network", "create", "--label=project=agentic-cli", NetworkName}, calls[1].args)
	})

	t.Run("create fails returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, client, "network inspect", "network create")

		// Act
		err := client.ensureEgressNetwork()

		// Assert
		assert.Error(t, err)
	})
}

func Test_createRunNetwork(t *testing.T) {
	client := newTestClient()

	t.Run("creates a labelled network the sweep can tie to its owner", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client)
		rs := toolRun{container: "agentic-claude-abc", network: "agentic-run-abc"}

		// Act
		err := client.createRunNetwork(rs)

		// Assert
		require.NoError(t, err)
		args := findCall(get(), "network", "create")
		assert.NotContains(t, args, "--internal")
		assert.Contains(t, args, "--label=project=agentic-cli")
		assert.Contains(t, args, "--label=agentic.owner=agentic-claude-abc")
		assert.Equal(t, "agentic-run-abc", args[len(args)-1])
	})

	t.Run("is internal with the enforcing proxy", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client)
		rs := toolRun{RunSpec: sandbox.RunSpec{Proxy: sandbox.ProxySpec{Mode: sandbox.ProxyEnforce}}, network: "agentic-run-abc"}

		// Act
		err := client.createRunNetwork(rs)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, findCall(get(), "network", "create"), "--internal")
	})

	t.Run("is internal with the monitoring proxy", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client)
		rs := toolRun{RunSpec: sandbox.RunSpec{Proxy: sandbox.ProxySpec{Mode: sandbox.ProxyMonitor}}, network: "agentic-run-abc"}

		// Act
		err := client.createRunNetwork(rs)

		// Assert
		require.NoError(t, err)
		assert.Contains(t, findCall(get(), "network", "create"), "--internal")
	})

	t.Run("create failure is an error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, client, "network create")

		// Act
		err := client.createRunNetwork(toolRun{network: "agentic-run-abc"})

		// Assert
		assert.ErrorContains(t, err, "create run network")
	})
}

func Test_networkSubnets(t *testing.T) {
	client := newTestClient()

	t.Run("returns every subnet", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "172.30.0.0/16 fd00::/64 \n", nil)

		// Act
		subnets, err := client.networkSubnets("agentic-run-abc")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"172.30.0.0/16", "fd00::/64"}, subnets)
	})

	t.Run("no subnet is an error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "\n", nil)

		// Act
		_, err := client.networkSubnets("agentic-run-abc")

		// Assert
		assert.ErrorContains(t, err, "has no subnet")
	})

	t.Run("inspect failure is an error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", fmt.Errorf("stub: inspect failed"))

		// Act
		_, err := client.networkSubnets("agentic-run-abc")

		// Assert
		assert.ErrorContains(t, err, "inspect agentic-run-abc subnets")
	})
}
