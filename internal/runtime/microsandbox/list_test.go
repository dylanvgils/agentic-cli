package microsandbox

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_ListRunning(t *testing.T) {
	t.Run("lists running agentic VMs, trimming the image to its tool repo and splitting it into namespace and tool", func(t *testing.T) {
		// Arrange
		out := `[
			{"name": "vm-1", "image": "myproject-claude:sha256-ab12", "status": "running"},
			{"name": "vm-2", "image": "agentic-copilot-dockerd:sha256-cd34", "status": "running"}
		]`
		c, args := stubRun(out, nil)

		// Act
		sandboxes, err := c.ListRunning()

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"ls", "--running", "--label=project=agentic-cli", "--format=json"}, *args)
		assert.Equal(t, []*Sandbox{
			{
				Name:      "vm-1",
				Image:     "myproject-claude",
				Namespace: "myproject",
				Tool:      "claude",
				Status:    "running",
			},
			{
				Name:      "vm-2",
				Image:     "agentic-copilot",
				Namespace: "agentic",
				Tool:      "copilot",
				Status:    "running",
			},
		}, sandboxes)
	})

	t.Run("returns the msb error", func(t *testing.T) {
		// Arrange
		c, _ := stubRun("", errors.New("boom"))

		// Act
		_, err := c.ListRunning()

		// Assert
		assert.ErrorContains(t, err, "list msb sandboxes: boom")
	})

	t.Run("refuses output that isn't JSON", func(t *testing.T) {
		// Arrange
		c, _ := stubRun("not json", nil)

		// Act
		_, err := c.ListRunning()

		// Assert
		assert.ErrorContains(t, err, "parse msb sandbox list")
	})
}

func TestPublishedPort_String(t *testing.T) {
	// Arrange
	port := PublishedPort{
		HostBind:  "127.0.0.1",
		HostPort:  8080,
		GuestPort: 80,
		Protocol:  "tcp",
	}

	// Act
	result := port.String()

	// Assert
	assert.Equal(t, "127.0.0.1:8080->80/tcp", result)
}

func TestClient_Ports(t *testing.T) {
	t.Run("returns the running sandbox's ports", func(t *testing.T) {
		// Arrange
		out := `{
			"active_config": {"network": {"ports": [
				{"guest_port": 3000, "host_bind": "127.0.0.1", "host_port": 3000, "protocol": "tcp"}
			]}},
			"name": "vm-1"
		}`
		c, args := stubRun(out, nil)

		// Act
		ports, err := c.Ports("vm-1")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []string{"inspect", "vm-1", "--format=json"}, *args)
		assert.Equal(t, []PublishedPort{
			{
				HostBind:  "127.0.0.1",
				HostPort:  3000,
				GuestPort: 3000,
				Protocol:  "tcp",
			},
		}, ports)
	})

	t.Run("returns the msb error", func(t *testing.T) {
		// Arrange
		c, _ := stubRun("", errors.New("boom"))

		// Act
		_, err := c.Ports("vm-1")

		// Assert
		assert.EqualError(t, err, "inspect msb sandbox vm-1: boom")
	})

	t.Run("refuses output that isn't JSON", func(t *testing.T) {
		// Arrange
		c, _ := stubRun("not json", nil)

		// Act
		_, err := c.Ports("vm-1")

		// Assert
		assert.ErrorContains(t, err, "parse msb sandbox vm-1")
	})
}
