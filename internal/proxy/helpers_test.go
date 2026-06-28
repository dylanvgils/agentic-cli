package proxy

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/url"
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
				defer conn.Close() //nolint:errcheck
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()

	host, port, err = net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	return host, port
}

// rawConnect issues a CONNECT to proxyAddr for target and returns the raw
// tunneled connection once the proxy reports success.
func rawConnect(t *testing.T, proxyAddr, target string) net.Conn {
	t.Helper()

	conn, err := net.Dial("tcp", proxyAddr)
	require.NoError(t, err)

	_, err = io.WriteString(conn, "CONNECT "+target+" HTTP/1.1\r\nHost: "+target+"\r\n\r\n")
	require.NoError(t, err)

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return conn
}

// connect issues a CONNECT and returns the response (used to assert non-200s).
func connect(t *testing.T, proxyAddr, target string) *http.Response {
	t.Helper()

	conn, err := net.Dial("tcp", proxyAddr)
	require.NoError(t, err)

	_, err = io.WriteString(conn, "CONNECT "+target+" HTTP/1.1\r\nHost: "+target+"\r\n\r\n")
	require.NoError(t, err)

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	return resp
}

// proxyGet performs a GET for target through the given proxy URL.
func proxyGet(t *testing.T, proxyURL, target string) *http.Response {
	t.Helper()

	parsed, err := http.NewRequest(http.MethodGet, target, nil)
	require.NoError(t, err)

	transport := &http.Transport{Proxy: func(*http.Request) (*url.URL, error) { return url.Parse(proxyURL) }}
	client := &http.Client{Transport: transport}
	resp, err := client.Do(parsed)
	require.NoError(t, err)
	return resp
}
