package proxy

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerConnect(t *testing.T) {
	t.Run("denied host returns 403 and logs deny", func(t *testing.T) {
		// Arrange
		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist(nil), NewLogger(&logBuf, nil, nil), false))
		t.Cleanup(proxy.Close)

		// Act
		resp := connect(t, proxy.Listener.Addr().String(), "evil.com:443")
		defer resp.Body.Close()

		// Assert
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Contains(t, logBuf.String(), `"decision":"deny"`)
		assert.Contains(t, logBuf.String(), `"host":"evil.com"`)
		assert.Contains(t, logBuf.String(), `"protocol":"https"`)
		assert.Contains(t, logBuf.String(), `"enforced":true`)
	})

	t.Run("allowed host tunnels bytes and logs allow", func(t *testing.T) {
		// Arrange
		upstreamHost, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)

		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(&logBuf, nil, nil), false))
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

	t.Run("monitor mode tunnels a denied host and logs deny unenforced", func(t *testing.T) {
		// Arrange
		upstreamHost, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)

		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist(nil), NewLogger(&logBuf, nil, nil), true))
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
		assert.Contains(t, logBuf.String(), `"decision":"deny"`)
		assert.Contains(t, logBuf.String(), `"enforced":false`)
	})
}

func TestServerHTTP(t *testing.T) {
	t.Run("denied host returns 403", func(t *testing.T) {
		// Arrange
		proxy := httptest.NewServer(NewServer(NewAllowlist(nil), NewLogger(io.Discard, nil, nil), false))
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

		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(io.Discard, nil, nil), false))
		t.Cleanup(proxy.Close)

		// Act
		resp := proxyGet(t, proxy.URL, upstream.URL)
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		// Assert
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "hello", string(body))
	})

	t.Run("monitor mode forwards a denied host and logs deny unenforced", func(t *testing.T) {
		// Arrange
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "hello")
		}))
		t.Cleanup(upstream.Close)

		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist(nil), NewLogger(&logBuf, nil, nil), true))
		t.Cleanup(proxy.Close)

		// Act
		resp := proxyGet(t, proxy.URL, upstream.URL)
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		// Assert
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "hello", string(body))
		assert.Contains(t, logBuf.String(), `"decision":"deny"`)
		assert.Contains(t, logBuf.String(), `"enforced":false`)
	})
}
