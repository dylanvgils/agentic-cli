package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckDaemon_returnsNil_whenDaemonRunning(t *testing.T) {
	// Arrange
	fakeDocker(t, `exit 0`)

	// Act
	err := CheckDaemon()

	// Assert
	require.NoError(t, err)
}

func TestCheckDaemon_returnsErrDaemonNotRunning_whenDaemonDown(t *testing.T) {
	// Arrange
	fakeDocker(t, `exit 1`)

	// Act
	err := CheckDaemon()

	// Assert
	assert.Equal(t, ErrDaemonNotRunning, err)
}
