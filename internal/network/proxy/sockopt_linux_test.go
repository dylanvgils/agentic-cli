package proxy

import (
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_setUserTimeout(t *testing.T) {
	// Arrange
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() }) //nolint:errcheck
	conn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck
	raw, err := conn.(*net.TCPConn).SyscallConn()
	require.NoError(t, err)

	// Act
	setUserTimeout(raw, 30*time.Second)

	// Assert
	var got int
	require.NoError(t, raw.Control(func(fd uintptr) {
		got, err = syscall.GetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_USER_TIMEOUT)
	}))
	require.NoError(t, err)
	assert.Equal(t, 30000, got)
}
