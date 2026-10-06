package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureNetwork(t *testing.T) {
	client := newTestClient()

	t.Run("existing network skips create", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client)

		// Act
		err := client.EnsureNetwork()

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
		err := client.EnsureNetwork()

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
		err := client.EnsureNetwork()

		// Assert
		assert.Error(t, err)
	})
}

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

func Test_networkSubnets(t *testing.T) {
	client := newTestClient()

	t.Run("returns every subnet", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "172.30.0.0/16 fd00::/64 \n", nil)

		// Act
		subnets, err := client.networkSubnets("agentic-proxy-abc")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"172.30.0.0/16", "fd00::/64"}, subnets)
	})

	t.Run("no subnet is an error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "\n", nil)

		// Act
		_, err := client.networkSubnets("agentic-proxy-abc")

		// Assert
		assert.ErrorContains(t, err, "has no subnet")
	})

	t.Run("inspect failure is an error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", fmt.Errorf("stub: inspect failed"))

		// Act
		_, err := client.networkSubnets("agentic-proxy-abc")

		// Assert
		assert.ErrorContains(t, err, "inspect agentic-proxy-abc subnets")
	})
}
