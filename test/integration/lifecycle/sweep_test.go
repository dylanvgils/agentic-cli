//go:build integration

package lifecycle

import (
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
	"github.com/stretchr/testify/assert"
)

// TestOrphanSweep checks a run removes sidecars and networks of crashed runs, and only those.
func TestOrphanSweep(t *testing.T) {
	// Well past the sweep's 5 minute grace period
	crashed := time.Now().Add(-time.Hour)

	t.Run("removes sidecar whose owner is gone", func(t *testing.T) {
		// Arrange
		sidecar := platform.FakeContainer(t).
			OwnedBy(platform.UniqueName(t)).
			StartedAt(crashed).
			Create()

		// Act
		platform.NewToolRun(t).Run("true")

		// Assert
		assert.False(t, sidecar.Exists())
	})

	t.Run("removes network whose owner is gone", func(t *testing.T) {
		// Arrange
		network := platform.FakeNetwork(t).
			OwnedBy(platform.UniqueName(t)).
			StartedAt(crashed).
			Create()

		// Act
		platform.NewToolRun(t).Run("true")

		// Assert
		assert.False(t, network.Exists())
	})

	t.Run("keeps sidecar within grace period", func(t *testing.T) {
		// Arrange
		sidecar := platform.FakeContainer(t).
			OwnedBy(platform.UniqueName(t)).
			StartedAt(time.Now()).
			Create()

		// Act
		platform.NewToolRun(t).Run("true")

		// Assert
		assert.True(t, sidecar.Exists())
	})

	t.Run("keeps sidecar whose owner exists", func(t *testing.T) {
		// Arrange
		owner := platform.FakeContainer(t).Create()
		sidecar := platform.FakeContainer(t).
			OwnedBy(owner.Name()).
			StartedAt(crashed).
			Create()

		// Act
		platform.NewToolRun(t).Run("true")

		// Assert
		assert.True(t, sidecar.Exists())
	})
}
