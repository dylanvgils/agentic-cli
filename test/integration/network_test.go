//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestToolNetwork checks the run's own network isolates the tool container from other runs and host containers.
func TestToolNetwork(t *testing.T) {
	t.Run("cannot reach another run's container", func(t *testing.T) {
		// Arrange
		otherRun := fakeNetwork(t).create()
		target := fakeContainer(t).onNetwork(otherRun.name).listening().create()

		// Act
		out := runInTool(t, "nc -z -w 3 "+target.ip()+" "+listenPort+" && echo reach=yes || echo reach=no")

		// Assert
		assert.Contains(t, out, "reach=no")
	})

	t.Run("cannot reach container on agentic-net", func(t *testing.T) {
		// Arrange
		target := fakeContainer(t).onNetwork("agentic-net").listening().create()

		// Act
		out := runInTool(t, "nc -z -w 3 "+target.ip()+" "+listenPort+" && echo reach=yes || echo reach=no")

		// Assert
		assert.Contains(t, out, "reach=no")
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
