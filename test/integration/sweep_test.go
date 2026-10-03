//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestOrphanSweep checks a run removes sidecars and networks of crashed runs, and only those.
func TestOrphanSweep(t *testing.T) {
	// Well past the sweep's 5 minute grace period
	crashed := time.Now().Add(-time.Hour)

	t.Run("removes sidecar whose owner is gone", func(t *testing.T) {
		// Arrange
		sidecar := fakeContainer(t).ownedBy(uniqueName(t)).startedAt(crashed).create()

		// Act
		runInTool(t, "true")

		// Assert
		assert.False(t, sidecar.exists())
	})

	t.Run("removes network whose owner is gone", func(t *testing.T) {
		// Arrange
		network := fakeNetwork(t).ownedBy(uniqueName(t)).startedAt(crashed).create()

		// Act
		runInTool(t, "true")

		// Assert
		assert.False(t, network.exists())
	})

	t.Run("keeps sidecar within grace period", func(t *testing.T) {
		// Arrange
		sidecar := fakeContainer(t).ownedBy(uniqueName(t)).startedAt(time.Now()).create()

		// Act
		runInTool(t, "true")

		// Assert
		assert.True(t, sidecar.exists())
	})

	t.Run("keeps sidecar whose owner exists", func(t *testing.T) {
		// Arrange
		owner := fakeContainer(t).create()
		sidecar := fakeContainer(t).ownedBy(owner.name).startedAt(crashed).create()

		// Act
		runInTool(t, "true")

		// Assert
		assert.True(t, sidecar.exists())
	})
}
