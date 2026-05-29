package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckDaemon(t *testing.T) {
	t.Run("returns nil when daemon running", func(t *testing.T) {
		// Arrange
		stubDocker(t, `exit 0`)

		// Act
		err := CheckDaemon()

		// Assert
		require.NoError(t, err)
	})

	t.Run("returns ErrDaemonNotRunning when daemon down", func(t *testing.T) {
		// Arrange
		stubDocker(t, `exit 1`)

		// Act
		err := CheckDaemon()

		// Assert
		assert.Equal(t, ErrDaemonNotRunning, err)
	})
}
