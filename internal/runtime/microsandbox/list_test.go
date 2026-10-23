package microsandbox

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ListRunning(t *testing.T) {
	t.Run("lists only running agentic VMs", func(t *testing.T) {
		// Arrange
		var gotArgs []string
		c := newTestClient(&fakeRunner{run: func(args ...string) (string, error) {
			gotArgs = args
			return "[]", nil
		}})

		// Act
		_, err := c.ListRunning()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"ls", "--running", "--label=project=agentic-cli", "--format=json"}, gotArgs)
	})

	t.Run("splits the image into namespace and tool", func(t *testing.T) {
		// Arrange
		out := `[
			{"name": "vm-1", "image": "myproject-claude:sha256-ab12", "status": "running"},
			{"name": "vm-2", "image": "agentic-copilot-dockerd:sha256-cd34", "status": "running"}
		]`
		c := newTestClient(&fakeRunner{run: func(...string) (string, error) { return out, nil }})

		// Act
		sandboxes, err := c.ListRunning()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []*Sandbox{
			{
				Name:      "vm-1",
				Image:     "myproject-claude:sha256-ab12",
				Namespace: "myproject",
				Tool:      "claude",
				Status:    "running",
			},
			{
				Name:      "vm-2",
				Image:     "agentic-copilot-dockerd:sha256-cd34",
				Namespace: "agentic",
				Tool:      "copilot",
				Status:    "running",
			},
		}, sandboxes)
	})

	t.Run("returns the msb error", func(t *testing.T) {
		// Arrange
		c := newTestClient(&fakeRunner{run: func(...string) (string, error) { return "", fmt.Errorf("boom") }})

		// Act
		_, err := c.ListRunning()

		// Assert
		assert.ErrorContains(t, err, "list msb sandboxes: boom")
	})

	t.Run("refuses output that isn't JSON", func(t *testing.T) {
		// Arrange
		c := newTestClient(&fakeRunner{run: func(...string) (string, error) { return "not json", nil }})

		// Act
		_, err := c.ListRunning()

		// Assert
		assert.ErrorContains(t, err, "parse msb sandbox list")
	})
}
