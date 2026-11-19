package sandbox

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// clearTerminalEnv unsets the forwarded terminal vars for the duration of the test.
func clearTerminalEnv(t *testing.T) {
	t.Helper()
	for _, key := range terminalEnvNames {
		t.Setenv(key, "")
	}
}

// listenLoopback listens on a free loopback port until the test ends and returns the listener and its port.
func listenLoopback(t *testing.T) (net.Listener, uint16) {
	t.Helper()
	listener, err := net.Listen("tcp", PortLoopback+":0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() }) //nolint:errcheck
	return listener, uint16(listener.Addr().(*net.TCPAddr).Port)
}
