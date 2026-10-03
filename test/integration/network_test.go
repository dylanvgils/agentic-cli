//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// connectScript prints whether a TCP connect to the address in $1 succeeds, within 3s.
const connectScript = `perl -MIO::Socket::INET -e 'print IO::Socket::INET->new(PeerAddr => shift, Timeout => 3) ? "reach=yes\n" : "reach=no\n"' `

// TestToolNetwork checks agentic-net isolates the tool container from containers on other networks.
func TestToolNetwork(t *testing.T) {
	t.Run("reaches container on agentic-net", func(t *testing.T) {
		// Arrange
		target := fakeContainer(t).onNetwork("agentic-net").listening().create()

		// Act
		out := runInTool(t, connectScript+target.addr())

		// Assert
		assert.Contains(t, out, "reach=yes")
	})

	t.Run("cannot reach container on default bridge", func(t *testing.T) {
		// Arrange
		target := fakeContainer(t).onNetwork("bridge").listening().create()

		// Act
		out := runInTool(t, connectScript+target.addr())

		// Assert
		assert.Contains(t, out, "reach=no")
	})
}
