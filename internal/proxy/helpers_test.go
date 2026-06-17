package proxy

import (
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// stubDefaultPorts temporarily replaces DefaultPorts for a test and restores it
// on cleanup, so tunnels to an ephemeral upstream port can be exercised.
func stubDefaultPorts(t *testing.T, ports ...string) {
	t.Helper()
	prev := DefaultPorts
	DefaultPorts = ports
	t.Cleanup(func() { DefaultPorts = prev })
}

// startEchoServer starts a TCP server that echoes back everything it receives
// and returns its host and port. It is torn down on cleanup.
func startEchoServer(t *testing.T) (host, port string) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()

	host, port, err = net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	return host, port
}
