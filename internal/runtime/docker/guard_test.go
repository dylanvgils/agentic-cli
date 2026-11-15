package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient_guardRun(t *testing.T) {
	client := newTestClient()

	t.Run("catches signals when asked", func(t *testing.T) {
		// Act
		guard := client.guardRun(t.TempDir(), testRunID, true)
		t.Cleanup(guard.stop)

		// Assert
		assert.NotNil(t, guard.signals)
	})

	t.Run("without signals there is nothing to interrupt", func(t *testing.T) {
		// Act
		guard := client.guardRun(t.TempDir(), testRunID, false)

		// Assert
		assert.Nil(t, guard.signals)
	})
}

func Test_runGuard_release(t *testing.T) {
	client := newTestClient()

	t.Run("tears its run down", func(t *testing.T) {
		// Arrange
		daemon := &fakeDaemon{
			containers: []ownedResource{owned(proxyContainerName(testRunID), "agentic-claude", testRunID, 0)},
			networks:   []ownedResource{owned(runNetworkName(testRunID), "agentic-claude", testRunID, 0)},
		}
		stubDaemon(t, client, daemon)
		guard := client.guardRun(t.TempDir(), testRunID, true)

		// Act
		guard.release()

		// Assert
		assert.Empty(t, daemon.names())
	})

	t.Run("a nil guard does nothing", func(t *testing.T) {
		// Arrange
		calls := stubDockerRunCapture(t, client)
		var guard *runGuard

		// Act
		guard.release()

		// Assert
		assert.Empty(t, calls())
	})
}
