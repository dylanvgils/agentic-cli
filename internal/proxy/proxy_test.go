package proxy

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerConnect(t *testing.T) {
	t.Run("denied host returns 403 and logs deny", func(t *testing.T) {
		// Arrange
		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist(nil), NewLogger(&logBuf, nil)))
		t.Cleanup(proxy.Close)

		// Act
		resp := connect(t, proxy.Listener.Addr().String(), "evil.com:443")
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Contains(t, logBuf.String(), `"decision":"deny"`)
		assert.Contains(t, logBuf.String(), `"host":"evil.com"`)
		assert.Contains(t, logBuf.String(), `"protocol":"https"`)
	})

	t.Run("allowed host tunnels bytes and logs allow", func(t *testing.T) {
		// Arrange
		upstreamHost, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)

		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(&logBuf, nil)))
		t.Cleanup(proxy.Close)

		// Act
		conn := rawConnect(t, proxy.Listener.Addr().String(), net.JoinHostPort(upstreamHost, upstreamPort))
		defer conn.Close()
		_, err := conn.Write([]byte("ping"))
		require.NoError(t, err)
		echo := make([]byte, 4)
		_, err = io.ReadFull(conn, echo)
		require.NoError(t, err)

		// Assert
		assert.Equal(t, "ping", string(echo))
		assert.Contains(t, logBuf.String(), `"decision":"allow"`)
		assert.Contains(t, logBuf.String(), `"protocol":"https"`)
	})
}

func TestServerHTTP(t *testing.T) {
	t.Run("denied host returns 403", func(t *testing.T) {
		// Arrange
		proxy := httptest.NewServer(NewServer(NewAllowlist(nil), NewLogger(io.Discard, nil)))
		t.Cleanup(proxy.Close)

		// Act
		resp := proxyGet(t, proxy.URL, "http://evil.com/")
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("allowed host is forwarded", func(t *testing.T) {
		// Arrange
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "hello")
		}))
		t.Cleanup(upstream.Close)
		upstreamHost, upstreamPort := splitHostPort(strings.TrimPrefix(upstream.URL, "http://"))
		stubDefaultPorts(t, upstreamPort)

		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(io.Discard, nil)))
		t.Cleanup(proxy.Close)

		// Act
		resp := proxyGet(t, proxy.URL, upstream.URL)
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		// Assert
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "hello", string(body))
	})
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
