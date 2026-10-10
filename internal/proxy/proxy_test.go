package proxy

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerConnect(t *testing.T) {
	t.Run("denied host returns 403 and logs deny", func(t *testing.T) {
		// Arrange
		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist(nil), NewLogger(&logBuf, nil, nil), false, nil))
		t.Cleanup(proxy.Close)

		// Act
		resp := connect(t, proxy.Listener.Addr().String(), "evil.test:443")
		defer resp.Body.Close() //nolint:errcheck

		// Assert
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Contains(t, logBuf.String(), `"decision":"deny"`)
		assert.Contains(t, logBuf.String(), `"host":"evil.test"`)
		assert.Contains(t, logBuf.String(), `"protocol":"https"`)
		assert.Contains(t, logBuf.String(), `"enforced":true`)
	})

	t.Run("allowed host tunnels bytes and logs allow", func(t *testing.T) {
		// Arrange
		stubAllowLocal(t)
		upstreamHost, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)

		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(&logBuf, nil, nil), false, nil))
		t.Cleanup(proxy.Close)

		// Act
		conn := rawConnect(t, proxy.Listener.Addr().String(), net.JoinHostPort(upstreamHost, upstreamPort))
		defer conn.Close() //nolint:errcheck
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
		stubAllowLocal(t)
		upstreamHost, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)

		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist(nil), NewLogger(&logBuf, nil, nil), true, nil))
		t.Cleanup(proxy.Close)

		// Act
		conn := rawConnect(t, proxy.Listener.Addr().String(), net.JoinHostPort(upstreamHost, upstreamPort))
		defer conn.Close() //nolint:errcheck
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

	t.Run("allowed host resolving to a blocked address returns 403 and logs the reason", func(t *testing.T) {
		// Arrange
		upstreamHost, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)

		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(&logBuf, nil, nil), false, nil))
		t.Cleanup(proxy.Close)

		// Act
		resp := connect(t, proxy.Listener.Addr().String(), net.JoinHostPort(upstreamHost, upstreamPort))
		defer resp.Body.Close() //nolint:errcheck

		// Assert
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Contains(t, logBuf.String(), `"decision":"deny","enforced":true,"reason":"blocked-address"`)
		assert.NotContains(t, logBuf.String(), `"decision":"allow"`)
	})

	t.Run("monitor mode still refuses a blocked address", func(t *testing.T) {
		// Arrange
		upstreamHost, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)

		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist(nil), NewLogger(&logBuf, nil, nil), true, nil))
		t.Cleanup(proxy.Close)

		// Act
		resp := connect(t, proxy.Listener.Addr().String(), net.JoinHostPort(upstreamHost, upstreamPort))
		defer resp.Body.Close() //nolint:errcheck

		// Assert
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Contains(t, logBuf.String(), `"decision":"deny","enforced":true,"reason":"blocked-address"`)
	})

	t.Run("wildcard match cannot reach a private address", func(t *testing.T) {
		// Arrange
		stubIsBlocked(t, func(_ netip.Addr, privateOK bool) bool { return !privateOK })
		_, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)

		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{".localhost"}), NewLogger(io.Discard, nil, nil), false, nil))
		t.Cleanup(proxy.Close)

		// Act
		resp := connect(t, proxy.Listener.Addr().String(), net.JoinHostPort("localhost", upstreamPort))
		defer resp.Body.Close() //nolint:errcheck

		// Assert
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("exact entry can reach a private address", func(t *testing.T) {
		// Arrange
		stubIsBlocked(t, func(_ netip.Addr, privateOK bool) bool { return !privateOK })
		_, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)

		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{"localhost"}), NewLogger(io.Discard, nil, nil), false, nil))
		t.Cleanup(proxy.Close)

		// Act
		conn := rawConnect(t, proxy.Listener.Addr().String(), net.JoinHostPort("localhost", upstreamPort))
		defer conn.Close() //nolint:errcheck
		_, err := conn.Write([]byte("ping"))
		require.NoError(t, err)
		echo := make([]byte, 4)
		_, err = io.ReadFull(conn, echo)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "ping", string(echo))
	})
}

func TestServerHTTP(t *testing.T) {
	t.Run("denied host returns 403", func(t *testing.T) {
		// Arrange
		proxy := httptest.NewServer(NewServer(NewAllowlist(nil), NewLogger(io.Discard, nil, nil), false, nil))
		t.Cleanup(proxy.Close)

		// Act
		resp := proxyGet(t, proxy.URL, "http://evil.test/")
		defer resp.Body.Close() //nolint:errcheck

		// Assert
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("allowed host is forwarded", func(t *testing.T) {
		// Arrange
		stubAllowLocal(t)
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "hello")
		}))
		t.Cleanup(upstream.Close)
		upstreamHost, upstreamPort := splitHostPort(strings.TrimPrefix(upstream.URL, "http://"))
		stubDefaultPorts(t, upstreamPort)

		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(io.Discard, nil, nil), false, nil))
		t.Cleanup(proxy.Close)

		// Act
		resp := proxyGet(t, proxy.URL, upstream.URL)
		defer resp.Body.Close() //nolint:errcheck
		body, _ := io.ReadAll(resp.Body)

		// Assert
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "hello", string(body))
	})

	t.Run("monitor mode forwards a denied host and logs deny unenforced", func(t *testing.T) {
		// Arrange
		stubAllowLocal(t)
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "hello")
		}))
		t.Cleanup(upstream.Close)

		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist(nil), NewLogger(&logBuf, nil, nil), true, nil))
		t.Cleanup(proxy.Close)

		// Act
		resp := proxyGet(t, proxy.URL, upstream.URL)
		defer resp.Body.Close() //nolint:errcheck
		body, _ := io.ReadAll(resp.Body)

		// Assert
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "hello", string(body))
		assert.Contains(t, logBuf.String(), `"decision":"deny"`)
		assert.Contains(t, logBuf.String(), `"enforced":false`)
	})

	t.Run("allowed host resolving to a blocked address returns 403 and logs the reason", func(t *testing.T) {
		// Arrange
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "hello")
		}))
		t.Cleanup(upstream.Close)
		upstreamHost, upstreamPort := splitHostPort(strings.TrimPrefix(upstream.URL, "http://"))
		stubDefaultPorts(t, upstreamPort)

		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(&logBuf, nil, nil), false, nil))
		t.Cleanup(proxy.Close)

		// Act
		resp := proxyGet(t, proxy.URL, upstream.URL)
		defer resp.Body.Close() //nolint:errcheck

		// Assert
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Contains(t, logBuf.String(), `"decision":"deny","enforced":true,"reason":"blocked-address"`)
	})
}
