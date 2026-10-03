//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestToolNetwork checks agentic-net isolates the tool container from containers on other networks.
func TestToolNetwork(t *testing.T) {
	t.Run("reaches container on agentic-net", func(t *testing.T) {
		// Arrange
		target := fakeContainer(t).onNetwork("agentic-net").listening().create()

		// Act
		out := runInTool(t, "nc -z -w 3 "+target.ip()+" "+listenPort+" && echo reach=yes || echo reach=no")

		// Assert
		assert.Contains(t, out, "reach=yes")
	})

	t.Run("cannot reach container on default bridge", func(t *testing.T) {
		// Arrange
		target := fakeContainer(t).onNetwork("bridge").listening().create()

		// Act
		out := runInTool(t, "nc -z -w 3 "+target.ip()+" "+listenPort+" && echo reach=yes || echo reach=no")

		// Assert
		assert.Contains(t, out, "reach=no")
	})
}
