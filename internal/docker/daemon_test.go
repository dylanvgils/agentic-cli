package docker

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckDaemon(t *testing.T) {
	client := newTestClient()

	t.Run("returns nil when daemon running", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", nil)

		// Act
		err := client.CheckDaemon()

		// Assert
		require.NoError(t, err)
	})

	t.Run("returns ErrDaemonNotRunning when daemon down", func(t *testing.T) {
		// Arrange
		stubDockerRunFixed(t, client, "", errors.New("stub: daemon down"))

		// Act
		err := client.CheckDaemon()

		// Assert
		assert.Equal(t, ErrDaemonNotRunning, err)
	})
}
