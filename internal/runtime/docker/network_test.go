package docker

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
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
		get := stubEgressNetwork(t, client, "0\n", nil)

		// Act
		err := client.RemoveNetwork()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"network", "rm", NetworkName}, findCall(get(), "network", "rm"))
	})

	t.Run("keeps the network while containers are attached", func(t *testing.T) {
		// Arrange
		get := stubEgressNetwork(t, client, "1\n", nil)

		// Act
		err := client.RemoveNetwork()

		// Assert
		require.NoError(t, err)
		assert.Nil(t, findCall(get(), "network", "rm"))
	})

	t.Run("rm fails propagates error", func(t *testing.T) {
		// Arrange
		stubEgressNetwork(t, client, "0\n", fmt.Errorf("stub: network rm failed"))

		// Act
		err := client.RemoveNetwork()

		// Assert
		assert.ErrorContains(t, err, "network rm failed")
	})
}

func TestSweepRunNetworks(t *testing.T) {
	// Arrange
	client := newTestClient()
	get := stubUnusedSweep(t, client, "", ownedRow("agentic-run-abc", "agentic-claude-gone", time.Hour))

	// Act
	removed, err := client.SweepRunNetworks()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"agentic-run-abc"}, removed)
	calls := get()
	assert.Contains(t, calls[1].args, "--filter=label=agentic.owner")
	assert.Equal(t, []string{"network", "rm", "agentic-run-abc"}, findCall(calls, "network", "rm"))
}

func Test_ensureEgressNetwork(t *testing.T) {
	client := newTestClient()

	t.Run("existing agentic-managed network skips create", func(t *testing.T) {
		// Arrange
		var calls [][]string
		stubDockerRun(t, client, func(args ...string) (string, error) {
			calls = append(calls, args)
			return "agentic-cli\n", nil
		})

		// Act
		err := client.ensureEgressNetwork()

		// Assert
		require.NoError(t, err)
		require.Len(t, calls, 1)
		assert.Equal(t, "inspect", calls[0][1])
	})

	t.Run("existing network without the agentic label is an error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "other-project\n", nil)

		// Act
		err := client.ensureEgressNetwork()

		// Assert
		assert.ErrorContains(t, err, "not an agentic-managed network")
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

	t.Run("network created concurrently is used", func(t *testing.T) {
		// Arrange
		stubDockerRun(t, client, raceAfterCheck("agentic-cli\n"))

		// Act
		err := client.ensureEgressNetwork()

		// Assert
		assert.NoError(t, err)
	})

	t.Run("network created concurrently without the agentic label is an error", func(t *testing.T) {
		// Arrange
		stubDockerRun(t, client, raceAfterCheck("other-project\n"))

		// Act
		err := client.ensureEgressNetwork()

		// Assert
		assert.ErrorContains(t, err, "not an agentic-managed network")
	})
}

func Test_createRunNetwork(t *testing.T) {
	client := newTestClient()

	t.Run("creates the run network", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, client)
		rs := toolRun{container: "agentic-claude-abc", network: "agentic-run-abc"}

		// Act
		err := client.createRunNetwork(rs)

		// Assert
		require.NoError(t, err)
		args := findCall(get(), "network", "create")
		assert.Equal(t, "agentic-run-abc", args[len(args)-1])
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

func Test_runNetworkArgs(t *testing.T) {
	t.Run("labels the network so the sweep can tie it to its owner", func(t *testing.T) {
		// Arrange
		rs := toolRun{container: "agentic-claude-abc", network: "agentic-run-abc"}

		// Act
		args := runNetworkArgs(rs)

		// Assert
		assert.Equal(t, []string{"network", "create"}, args[:2])
		assert.NotContains(t, args, "--internal")
		assert.NotContains(t, args, "--opt="+inhibitIPv4Opt, "egress without the proxy goes through the gateway")
		assert.Contains(t, args, "--label=project=agentic-cli")
		assert.Contains(t, args, "--label=agentic.owner=agentic-claude-abc")
		assert.Equal(t, "agentic-run-abc", args[len(args)-1])
	})

	t.Run("is internal with the enforcing proxy", func(t *testing.T) {
		// Arrange
		rs := toolRun{RunSpec: sandbox.RunSpec{Proxy: sandbox.ProxySpec{Mode: sandbox.ProxyEnforce}}, network: "agentic-run-abc"}

		// Act
		args := runNetworkArgs(rs)

		// Assert
		assert.Contains(t, args, "--internal")
		assert.Contains(t, args, "--opt="+inhibitIPv4Opt, "no host gateway to reach host services through")
		assert.Contains(t, args, "--ipv6=false", "no IPv6 host gateway either")
	})

	t.Run("is internal with the monitoring proxy", func(t *testing.T) {
		// Arrange
		rs := toolRun{RunSpec: sandbox.RunSpec{Proxy: sandbox.ProxySpec{Mode: sandbox.ProxyMonitor}}, network: "agentic-run-abc"}

		// Act
		args := runNetworkArgs(rs)

		// Assert
		assert.Contains(t, args, "--internal")
	})
}

// raceAfterCheck returns a docker run stub where agentic-net is missing on the first inspect, then created by someone else
// with project before the create, which fails.
func raceAfterCheck(project string) func(args ...string) (string, error) {
	inspects := 0
	return func(args ...string) (string, error) {
		switch args[1] {
		case "inspect":
			inspects++
			if inspects == 1 {
				return "", errors.New("network agentic-net not found")
			}
			return project, nil
		case "create":
			return "", errors.New("network with name agentic-net already exists")
		}
		return "", nil
	}
}
