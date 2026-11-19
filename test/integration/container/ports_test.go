//go:build integration

package container

import (
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
)

// serveScript answers every request on port 8080 with "forwarded".
const serveScript = `while true; do printf 'HTTP/1.0 200 OK\r\n\r\nforwarded\n' | nc -N -l 8080; done`

func TestPorts(t *testing.T) {
	// Arrange
	skipWithRemoteDaemon(t)
	hostPort := freePort(t)
	run := platform.NewToolRun(t).WithPort(hostPort + ":8080")

	run.During(func() {
		platform.Docker(t, "exec", "--detach", run.Containers()[0], "sh", "-c", serveScript)

		// Act + Assert
		assert.Eventually(t, func() bool {
			return fetchLocal(hostPort) == "forwarded"
		}, 30*time.Second, 500*time.Millisecond, "the forwarded port never answered")
	})
}
