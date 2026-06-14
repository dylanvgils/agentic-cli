package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureNetwork(t *testing.T) {
	t.Run("existing network skips create", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t)

		// Act
		err := EnsureNetwork()

		// Assert
		require.NoError(t, err)
		calls := get()
		require.Len(t, calls, 1)
		assert.Equal(t, []string{"network", "inspect", NetworkName}, calls[0].args)
	})

	t.Run("missing network creates with label", func(t *testing.T) {
		// Arrange
		get := stubDockerRunCapture(t, "network inspect")

		// Act
		err := EnsureNetwork()

		// Assert
		require.NoError(t, err)
		calls := get()
		require.Len(t, calls, 2)
		assert.Equal(t, []string{"network", "inspect", NetworkName}, calls[0].args)
		assert.Equal(t, []string{"network", "create", "--label=project=agentic-cli", NetworkName}, calls[1].args)
	})

	t.Run("create fails returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, "network inspect", "network create")

		// Act
		err := EnsureNetwork()

		// Assert
		assert.Error(t, err)
	})
}

func TestRemoveNetwork(t *testing.T) {
	t.Run("network does not exist returns nil", func(t *testing.T) {
		// Arrange
		stubDockerRunCapture(t, "network inspect")

		// Act
		err := RemoveNetwork()

		// Assert
		require.NoError(t, err)
	})

	t.Run("wrong label returns error", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, "other-project\n", nil)

		// Act
		err := RemoveNetwork()

		// Assert
		assert.ErrorContains(t, err, "not an agentic-managed network")
	})

	t.Run("agentic-managed network calls rm", func(t *testing.T) {
		// Arrange
		var calls []dockerCall
		stubDockerRun(t, func(args ...string) (string, error) {
			calls = append(calls, dockerCall{args: args})
			if args[0] == "network" && args[1] == "inspect" {
				return "agentic-cli\n", nil
			}
			return "", nil
		})

		// Act
		err := RemoveNetwork()

		// Assert
		require.NoError(t, err)
		require.Len(t, calls, 2)
		assert.Equal(t, "inspect", calls[0].args[1])
		assert.Equal(t, []string{"network", "rm", NetworkName}, calls[1].args)
	})

	t.Run("rm fails propagates error", func(t *testing.T) {
		// Arrange
		stubDockerRun(t, func(args ...string) (string, error) {
			if args[0] == "network" && args[1] == "inspect" {
				return "agentic-cli\n", nil
			}
			return "", fmt.Errorf("stub: network rm failed")
		})

		// Act
		err := RemoveNetwork()

		// Assert
		assert.ErrorContains(t, err, "network rm failed")
	})
}
