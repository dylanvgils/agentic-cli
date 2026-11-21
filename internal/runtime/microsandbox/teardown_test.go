package microsandbox

import (
	"errors"
	"testing"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Teardown(t *testing.T) {
	t.Run("stops the run's VMs", func(t *testing.T) {
		// Arrange
		var calls [][]string
		client := newTestClient(&fakeRunner{run: func(args ...string) (string, error) {
			calls = append(calls, args)
			if args[0] == "ls" {
				return `[{"name":"vm-a","image":"agentic-claude","status":"running"}]`, nil
			}
			return "", nil
		}})

		// Act
		err := client.Teardown(testRunID)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, [][]string{
			{
				"ls",
				"--running",
				"--label=agentic.run=" + string(testRunID),
				"--format=json",
			},
			{
				"stop",
				"vm-a",
			},
		}, calls)
	})

	t.Run("a run already torn down is a no-op", func(t *testing.T) {
		// Arrange
		client := newTestClient(&fakeRunner{run: func(...string) (string, error) { return "[]", nil }})

		// Act
		first := client.Teardown(testRunID)
		second := client.Teardown(testRunID)

		// Assert
		require.NoError(t, first)
		require.NoError(t, second)
	})

	t.Run("a failed listing is returned", func(t *testing.T) {
		// Arrange
		client := newTestClient(&fakeRunner{run: func(...string) (string, error) { return "", errors.New("stub: msb down") }})

		// Act
		err := client.Teardown(testRunID)

		// Assert
		require.EqualError(t, err, "list the VMs of run "+string(testRunID)+": stub: msb down")
	})

	t.Run("refuses an invalid run id", func(t *testing.T) {
		// Arrange
		client := newTestClient(&fakeRunner{})

		// Act
		err := client.Teardown(sandbox.RunID("../x"))

		// Assert
		require.EqualError(t, err, `invalid run id "../x"`)
	})
}
