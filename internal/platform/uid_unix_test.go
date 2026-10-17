//go:build !windows

package platform

import (
	"os/user"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostUser(t *testing.T) {
	// Arrange
	current, err := user.Current()
	require.NoError(t, err)

	// Act
	u := HostUser()

	// Assert
	assert.Equal(t, current.Uid, strconv.Itoa(u.UID))
	assert.Equal(t, current.Gid, strconv.Itoa(u.GID))
}
