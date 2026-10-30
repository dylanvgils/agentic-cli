//go:build integration

package network

import (
	"testing"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
)

// hostListenPort is the fake host service's port, apart from platform.ListenPort as it shares the daemon's network namespace.
const hostListenPort = "18080"

// TestToolNetwork checks the run's own network isolates the tool container from other runs and host containers.
func TestToolNetwork(t *testing.T) {
	t.Run("cannot reach another run's container", func(t *testing.T) {
		// Arrange
		otherRun := platform.FakeNetwork(t).Create()
		target := platform.FakeContainer(t).
			OnNetwork(otherRun.Name()).
			Listening().
			Create()

		// Act
		out := platform.NewToolRun(t).Run(reachScript(target.IP(), platform.ListenPort))

		// Assert
		assert.Contains(t, out, "reach=no")
	})

	t.Run("cannot reach container on agentic-net", func(t *testing.T) {
		// Arrange
		target := platform.FakeContainer(t).
			OnNetwork("agentic-net").
			Listening().
			Create()

		// Act
		out := platform.NewToolRun(t).Run(reachScript(target.IP(), platform.ListenPort))

		// Assert
		assert.Contains(t, out, "reach=no")
	})

	t.Run("cannot reach container on default bridge", func(t *testing.T) {
		// Arrange
		target := platform.FakeContainer(t).
			OnNetwork("bridge").
			Listening().
			Create()

		// Act
		out := platform.NewToolRun(t).Run(reachScript(target.IP(), platform.ListenPort))

		// Assert
		assert.Contains(t, out, "reach=no")
	})

	t.Run("can reach the Docker host through the gateway", func(t *testing.T) {
		// Arrange - also proves gatewayScript finds the host, so the proxy case below can't pass by accident
		platform.FakeContainer(t).
			OnNetwork("host").
			ListeningOn(hostListenPort).
			Create()

		// Act
		out := platform.NewToolRun(t).Run(gatewayScript(hostListenPort))

		// Assert
		assert.Contains(t, out, "reach=yes")
	})
}

// TestProxyNetwork checks a --proxy run's internal network leaves the proxy as the tool's only way out.
func TestProxyNetwork(t *testing.T) {
	t.Run("cannot reach another run's container", func(t *testing.T) {
		// Arrange
		otherRun := platform.FakeNetwork(t).Create()
		target := platform.FakeContainer(t).
			OnNetwork(otherRun.Name()).
			Listening().
			Create()

		// Act
		out := platform.NewToolRun(t).
			WithProxy().
			Run(reachScript(target.IP(), platform.ListenPort))

		// Assert
		assert.Contains(t, out, "reach=no")
	})

	t.Run("cannot reach container on agentic-net directly", func(t *testing.T) {
		// Arrange
		target := platform.FakeContainer(t).
			OnNetwork("agentic-net").
			Listening().
			Create()

		// Act
		out := platform.NewToolRun(t).
			WithProxy().
			Run(reachScript(target.IP(), platform.ListenPort))

		// Assert
		assert.Contains(t, out, "reach=no")
	})

	t.Run("cannot reach the Docker host through the gateway", func(t *testing.T) {
		// Arrange
		platform.FakeContainer(t).
			OnNetwork("host").
			ListeningOn(hostListenPort).
			Create()

		// Act
		out := platform.NewToolRun(t).
			WithProxy().
			Run(gatewayScript(hostListenPort))

		// Assert
		assert.Contains(t, out, "reach=no")
	})
}

// reachScript prints reach=yes when a TCP connection to ip:port succeeds, reach=no otherwise.
func reachScript(ip, port string) string {
	return "nc -z -w 3 " + ip + " " + port + " && echo reach=yes || echo reach=no"
}

// gatewayScript is reachScript for the run network's gateway address, the first one in the tool's subnet.
func gatewayScript(port string) string {
	return `gw=$(hostname -i | awk '{print $1}' | sed 's/\.[0-9]*$/.1/'); echo "gateway=$gw"; ` + reachScript("$gw", port)
}
