package proxy

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

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

	t.Run("failed hijack returns a fixed 500 and logs the error once", func(t *testing.T) {
		// Arrange
		upstreamHost, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)
		var logBuf bytes.Buffer
		server := allowLocal(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(&logBuf, nil, nil), false, nil))
		target := net.JoinHostPort(upstreamHost, upstreamPort)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodConnect, "http://"+target, nil)
		req.Host = target

		// Act
		server.ServeHTTP(w, req)

		// Assert
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Equal(t, proxyFailedText+"\n", w.Body.String())
		assert.Contains(t, logBuf.String(), `"reason":"proxy-failed","error":"connection does not support hijacking"`)
		assert.Equal(t, 1, strings.Count(logBuf.String(), "\n"))
	})

	t.Run("allowed host tunnels bytes and logs allow", func(t *testing.T) {
		// Arrange
		upstreamHost, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)

		var logBuf bytes.Buffer
		proxy := httptest.NewServer(allowLocal(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(&logBuf, nil, nil), false, nil)))
		t.Cleanup(proxy.Close)
		hello := clientHello(t, "")

		// Act
		conn := rawConnect(t, proxy.Listener.Addr().String(), net.JoinHostPort(upstreamHost, upstreamPort))
		defer conn.Close() //nolint:errcheck
		echo, err := echoed(t, conn, hello)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, hello, echo)
		assert.Contains(t, logBuf.String(), `"decision":"allow"`)
		assert.Contains(t, logBuf.String(), `"protocol":"https"`)
	})

	t.Run("bytes sent with the connect reach the upstream", func(t *testing.T) {
		// Arrange
		upstreamHost, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)
		target := net.JoinHostPort(upstreamHost, upstreamPort)

		proxy := httptest.NewServer(allowLocal(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(nil, nil, nil), false, nil)))
		t.Cleanup(proxy.Close)

		conn, err := net.Dial("tcp", proxy.Listener.Addr().String())
		require.NoError(t, err)
		defer conn.Close() //nolint:errcheck
		require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
		hello := clientHello(t, "")

		// Act
		_, err = io.WriteString(conn, "CONNECT "+target+" HTTP/1.1\r\nHost: "+target+"\r\n\r\n"+string(hello))
		require.NoError(t, err)
		reader := bufio.NewReader(conn)
		resp, err := http.ReadResponse(reader, nil)
		require.NoError(t, err)
		echo := make([]byte, len(hello))
		_, err = io.ReadFull(reader, echo)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, hello, echo)
	})

	t.Run("failed upstream returns a fixed 502 and logs the error", func(t *testing.T) {
		// Arrange
		upstreamHost, upstreamPort := closedPort(t)
		stubDefaultPorts(t, upstreamPort)

		var logBuf bytes.Buffer
		proxy := httptest.NewServer(allowLocal(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(&logBuf, nil, nil), false, nil)))
		t.Cleanup(proxy.Close)

		// Act
		resp := connect(t, proxy.Listener.Addr().String(), net.JoinHostPort(upstreamHost, upstreamPort))
		defer resp.Body.Close() //nolint:errcheck
		body, _ := io.ReadAll(resp.Body)

		// Assert
		assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
		assert.Equal(t, "upstream unreachable via agentic proxy\n", string(body))
		assert.Contains(t, logBuf.String(), `"reason":"upstream-failed","error":"dial tcp `)
	})

	t.Run("monitor mode tunnels a denied host and logs deny unenforced", func(t *testing.T) {
		// Arrange
		upstreamHost, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)

		var logBuf bytes.Buffer
		proxy := httptest.NewServer(allowLocal(NewServer(NewAllowlist(nil), NewLogger(&logBuf, nil, nil), true, nil)))
		t.Cleanup(proxy.Close)
		hello := clientHello(t, "")

		// Act
		conn := rawConnect(t, proxy.Listener.Addr().String(), net.JoinHostPort(upstreamHost, upstreamPort))
		defer conn.Close() //nolint:errcheck
		echo, err := echoed(t, conn, hello)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, hello, echo)
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
		_, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)

		proxy := httptest.NewServer(withAddrGuard(NewServer(NewAllowlist([]string{".localhost"}), NewLogger(io.Discard, nil, nil), false, nil), func(_ netip.Addr, privateOK bool) bool { return !privateOK }))
		t.Cleanup(proxy.Close)

		// Act
		resp := connect(t, proxy.Listener.Addr().String(), net.JoinHostPort("localhost", upstreamPort))
		defer resp.Body.Close() //nolint:errcheck

		// Assert
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("exact entry can reach a private address", func(t *testing.T) {
		// Arrange
		_, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)

		proxy := httptest.NewServer(withAddrGuard(NewServer(NewAllowlist([]string{"localhost"}), NewLogger(io.Discard, nil, nil), false, nil), func(_ netip.Addr, privateOK bool) bool { return !privateOK }))
		t.Cleanup(proxy.Close)
		hello := clientHello(t, "localhost")

		// Act
		conn := rawConnect(t, proxy.Listener.Addr().String(), net.JoinHostPort("localhost", upstreamPort))
		defer conn.Close() //nolint:errcheck
		echo, err := echoed(t, conn, hello)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, hello, echo)
	})

	t.Run("tls server name of another host closes the tunnel and logs the mismatch", func(t *testing.T) {
		// Arrange
		upstreamHost, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)

		logBuf := &syncBuffer{}
		proxy := httptest.NewServer(allowLocal(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(logBuf, nil, nil), false, nil)))
		t.Cleanup(proxy.Close)
		hello := clientHello(t, "other.example.test")

		// Act
		conn := rawConnect(t, proxy.Listener.Addr().String(), net.JoinHostPort(upstreamHost, upstreamPort))
		defer conn.Close() //nolint:errcheck
		_, err := echoed(t, conn, hello)

		// Assert
		assert.ErrorIs(t, err, io.EOF)
		assert.Contains(t, logBuf.String(), `"decision":"deny","enforced":true,"reason":"sni-mismatch","error":"tls server name \"other.example.test\""`)
		assert.NotContains(t, logBuf.String(), `"decision":"allow"`)
	})

	t.Run("tunnel without tls closes and logs the mismatch", func(t *testing.T) {
		// Arrange
		upstreamHost, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)

		logBuf := &syncBuffer{}
		proxy := httptest.NewServer(allowLocal(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(logBuf, nil, nil), false, nil)))
		t.Cleanup(proxy.Close)

		// Act
		conn := rawConnect(t, proxy.Listener.Addr().String(), net.JoinHostPort(upstreamHost, upstreamPort))
		defer conn.Close() //nolint:errcheck
		_, err := echoed(t, conn, []byte("GET / HTTP/1.1\r\nHost: other.example.test\r\n\r\n"))

		// Assert
		assert.ErrorIs(t, err, io.EOF)
		assert.Contains(t, logBuf.String(), `"decision":"deny","enforced":true,"reason":"sni-mismatch","error":"read tls client hello: `)
	})

	t.Run("monitor mode tunnels another tls server name and logs the mismatch unenforced", func(t *testing.T) {
		// Arrange
		upstreamHost, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)

		logBuf := &syncBuffer{}
		proxy := httptest.NewServer(allowLocal(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(logBuf, nil, nil), true, nil)))
		t.Cleanup(proxy.Close)
		hello := clientHello(t, "other.example.test")

		// Act
		conn := rawConnect(t, proxy.Listener.Addr().String(), net.JoinHostPort(upstreamHost, upstreamPort))
		defer conn.Close() //nolint:errcheck
		echo, err := echoed(t, conn, hello)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, hello, echo)
		assert.Contains(t, logBuf.String(), `"decision":"deny","enforced":false,"reason":"sni-mismatch"`)
	})

	t.Run("client closing before its hello logs client-closed", func(t *testing.T) {
		// Arrange
		upstreamHost, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)

		logBuf := &syncBuffer{}
		proxy := httptest.NewServer(allowLocal(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(logBuf, nil, nil), false, nil)))
		t.Cleanup(proxy.Close)

		// Act
		conn := rawConnect(t, proxy.Listener.Addr().String(), net.JoinHostPort(upstreamHost, upstreamPort))
		require.NoError(t, conn.Close())

		// Assert
		assert.Eventually(t, func() bool {
			return strings.Contains(logBuf.String(), `"decision":"allow","enforced":true,"reason":"client-closed"`)
		}, 5*time.Second, 10*time.Millisecond)
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

	t.Run("https absolute-form is logged on port 443 with the normalized host", func(t *testing.T) {
		// Arrange
		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist(nil), NewLogger(&logBuf, nil, nil), false, nil))
		t.Cleanup(proxy.Close)
		conn, err := net.Dial("tcp", proxy.Listener.Addr().String())
		require.NoError(t, err)
		defer conn.Close() //nolint:errcheck

		// Act
		_, err = io.WriteString(conn, "GET https://Evil.Test./ HTTP/1.1\r\nHost: Evil.Test.\r\n\r\n")
		require.NoError(t, err)
		resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck

		// Assert
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Contains(t, logBuf.String(), `"host":"evil.test","port":"443"`)
	})

	t.Run("allowed host is forwarded", func(t *testing.T) {
		// Arrange
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "hello")
		}))
		t.Cleanup(upstream.Close)
		upstreamHost, upstreamPort := splitHostPort(strings.TrimPrefix(upstream.URL, "http://"))
		stubDefaultPorts(t, upstreamPort)

		proxy := httptest.NewServer(allowLocal(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(io.Discard, nil, nil), false, nil)))
		t.Cleanup(proxy.Close)

		// Act
		resp := proxyGet(t, proxy.URL, upstream.URL)
		defer resp.Body.Close() //nolint:errcheck
		body, _ := io.ReadAll(resp.Body)

		// Assert
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "hello", string(body))
	})

	t.Run("failed upstream returns a fixed 502 and logs the error", func(t *testing.T) {
		// Arrange
		upstreamHost, upstreamPort := closedPort(t)
		stubDefaultPorts(t, upstreamPort)

		var logBuf bytes.Buffer
		proxy := httptest.NewServer(allowLocal(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(&logBuf, nil, nil), false, nil)))
		t.Cleanup(proxy.Close)

		// Act
		resp := proxyGet(t, proxy.URL, "http://"+net.JoinHostPort(upstreamHost, upstreamPort))
		defer resp.Body.Close() //nolint:errcheck
		body, _ := io.ReadAll(resp.Body)

		// Assert
		assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
		assert.Equal(t, "upstream unreachable via agentic proxy\n", string(body))
		assert.Contains(t, logBuf.String(), `"reason":"upstream-failed","error":"dial tcp `)
	})

	t.Run("monitor mode forwards a denied host and logs deny unenforced", func(t *testing.T) {
		// Arrange
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "hello")
		}))
		t.Cleanup(upstream.Close)

		var logBuf bytes.Buffer
		proxy := httptest.NewServer(allowLocal(NewServer(NewAllowlist(nil), NewLogger(&logBuf, nil, nil), true, nil)))
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

func Test_Server_logClientClosed(t *testing.T) {
	// Arrange
	var logBuf bytes.Buffer
	s := NewServer(NewAllowlist(nil), NewLogger(&logBuf, nil, nil), false, nil)
	entry := Entry{Protocol: ProtocolHTTPS, Host: "api.example.test", Port: "443", Decision: DecisionAllow, Enforced: true}

	// Act
	s.logClientClosed(entry, errors.New("test-error"))

	// Assert
	assert.Contains(t, logBuf.String(), `"host":"api.example.test"`)
	assert.Contains(t, logBuf.String(), `"reason":"client-closed","error":"test-error"`)
}

func Test_defaultPort(t *testing.T) {
	cases := map[string]struct{ scheme, want string }{
		"http":              {"http", "80"},
		"https":             {"https", "443"},
		"https in any case": {"HTTPS", "443"},
		"no scheme":         {"", "80"},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			port := defaultPort(c.scheme)

			// Assert
			assert.Equal(t, c.want, port)
		})
	}
}
