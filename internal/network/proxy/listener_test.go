package proxy

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_oneConnListener_Accept(t *testing.T) {
	// Arrange
	client, server := net.Pipe()
	defer client.Close() //nolint:errcheck
	listener := newOneConnListener(server)

	// Act
	conn, err := listener.Accept()

	// Assert
	require.NoError(t, err)
	assert.Same(t, server, conn.(*closeNotifyConn).Conn)
}

func Test_oneConnListener_Close(t *testing.T) {
	// Arrange
	client, server := net.Pipe()
	defer client.Close() //nolint:errcheck
	defer server.Close() //nolint:errcheck
	listener := newOneConnListener(server)
	_ = listener.Close()

	// Act
	err := listener.Close()

	// Assert
	assert.NoError(t, err)
}

func Test_closeNotifyConn_Close(t *testing.T) {
	// Arrange
	client, server := net.Pipe()
	defer client.Close() //nolint:errcheck
	listener := newOneConnListener(server)
	conn, err := listener.Accept()
	require.NoError(t, err)

	// Act
	_ = conn.Close()

	// Assert
	_, err = listener.Accept()
	assert.ErrorIs(t, err, net.ErrClosed)
	_, err = client.Write([]byte("x"))
	assert.Error(t, err, "the wrapped conn is closed")
}
