package proxy

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hijackRecorder is a ResponseWriter whose Hijack hands out conn.
type hijackRecorder struct {
	*httptest.ResponseRecorder
	conn net.Conn
}

func (h hijackRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return h.conn, bufio.NewReadWriter(bufio.NewReader(h.conn), bufio.NewWriter(h.conn)), nil
}

func Test_Server_acceptTunnel(t *testing.T) {
	entry := Entry{Host: "api.example.test", Decision: DecisionAllow}

	t.Run("answers 200 on the hijacked conn", func(t *testing.T) {
		// Arrange
		client, server := net.Pipe()
		defer client.Close() //nolint:errcheck
		s := NewServer(NewAllowlist(nil), NewLogger(nil), false, nil)
		reply := make(chan string, 1)
		go func() {
			buf := make([]byte, len(connectEstablished))
			_, _ = io.ReadFull(client, buf)
			reply <- string(buf)
		}()

		// Act
		conn, ok := s.acceptTunnel(hijackRecorder{httptest.NewRecorder(), server}, entry)

		// Assert
		require.True(t, ok)
		defer conn.Close() //nolint:errcheck
		assert.Equal(t, connectEstablished, <-reply)
	})

	t.Run("client gone before the reply is logged without a reason", func(t *testing.T) {
		// Arrange
		client, server := net.Pipe()
		_ = client.Close()
		var logBuf bytes.Buffer
		s := NewServer(NewAllowlist(nil), NewLogger(&logBuf), false, nil)

		// Act
		_, ok := s.acceptTunnel(hijackRecorder{httptest.NewRecorder(), server}, entry)

		// Assert
		assert.False(t, ok)
		assert.Contains(t, logBuf.String(), `"host":"api.example.test"`)
		assert.NotContains(t, logBuf.String(), `"reason"`)
	})
}

func Test_bufferedConn_Read(t *testing.T) {
	// Arrange
	client, server := net.Pipe()
	defer client.Close() //nolint:errcheck
	reader := bufio.NewReader(strings.NewReader("buffered|"))
	_, _ = reader.Peek(1)
	conn := &bufferedConn{Conn: server, reader: reader}
	go func() { _, _ = client.Write([]byte("conn")) }()

	// Act
	got := make([]byte, len("buffered|conn"))
	_, err := io.ReadFull(conn, got)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "buffered|conn", string(got))
}

func Test_splice(t *testing.T) {
	// Arrange
	left, a := net.Pipe()
	b, right := net.Pipe()
	done := make(chan struct{})
	go func() {
		splice(a, b)
		close(done)
	}()

	// Act
	go func() { _, _ = left.Write([]byte("ping")) }()
	fromLeft := make([]byte, 4)
	_, errRight := io.ReadFull(right, fromLeft)
	go func() { _, _ = right.Write([]byte("pong")) }()
	fromRight := make([]byte, 4)
	_, errLeft := io.ReadFull(left, fromRight)
	_ = left.Close()
	_ = a.Close()
	<-done
	_ = b.Close()
	_ = right.Close()

	// Assert
	require.NoError(t, errRight)
	require.NoError(t, errLeft)
	assert.Equal(t, "ping", string(fromLeft))
	assert.Equal(t, "pong", string(fromRight))
}

func Test_clientClosed(t *testing.T) {
	t.Run("hang-ups count as closed", func(t *testing.T) {
		for _, err := range []error{
			fmt.Errorf("client handshake for example.test: %w", io.EOF),
			io.ErrUnexpectedEOF,
			&net.OpError{Op: "read", Err: os.NewSyscallError("read", syscall.ECONNRESET)},
			syscall.EPIPE,
		} {
			// Act
			closed := clientClosed(err)

			// Assert
			assert.True(t, closed, err.Error())
		}
	})

	t.Run("tls error is not closed", func(t *testing.T) {
		// Arrange
		err := fmt.Errorf("client handshake for example.test: %w", tls.AlertError(20))

		// Act
		closed := clientClosed(err)

		// Assert
		assert.False(t, closed)
	})
}
