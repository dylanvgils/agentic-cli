package proxy

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerConnect(t *testing.T) {
	for _, tc := range []struct {
		name    string
		guard   func(*Server) *Server
		hosts   []string
		monitor bool
		target  string
		wantLog string
	}{
		{
			name:    "denied host returns 403 and logs deny",
			target:  "evil.test",
			wantLog: `"protocol":"https","host":"evil.test","port":"%s","decision":"deny","enforced":true}`,
		},
		{
			name:    "allowed host resolving to a blocked address returns 403 and logs the reason",
			hosts:   []string{"127.0.0.1"},
			target:  "127.0.0.1",
			wantLog: `"decision":"deny","enforced":true,"reason":"blocked-address"`,
		},
		{
			name:    "monitor mode still refuses a blocked address",
			monitor: true,
			target:  "127.0.0.1",
			wantLog: `"decision":"deny","enforced":true,"reason":"blocked-address"`,
		},
		{
			name:    "wildcard match cannot reach a private address",
			guard:   privateOnlyExact,
			hosts:   []string{".localhost"},
			target:  "localhost",
			wantLog: `"reason":"blocked-address"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			_, upstreamPort := startEchoServer(t)
			stubDefaultPorts(t, upstreamPort)
			var logBuf syncBuffer
			server := NewServer(NewAllowlist(hostRules(tc.hosts...)), NewLogger(&logBuf), tc.monitor, nil)
			if tc.guard != nil {
				server = tc.guard(server)
			}
			proxy := startProxy(t, server)

			// Act
			resp := connect(t, proxy.Listener.Addr().String(), net.JoinHostPort(tc.target, upstreamPort))
			defer resp.Body.Close() //nolint:errcheck

			// Assert
			assert.Equal(t, http.StatusForbidden, resp.StatusCode)
			assert.Contains(t, logBuf.String(), strings.ReplaceAll(tc.wantLog, "%s", upstreamPort))
			assert.NotContains(t, logBuf.String(), `"decision":"allow"`)
		})
	}

	for _, tc := range []struct {
		name       string
		guard      func(*Server) *Server
		hosts      []string
		monitor    bool
		target     string
		serverName string
		wantLog    string
	}{
		{
			name:    "allowed host tunnels bytes and logs allow",
			guard:   allowLocal,
			hosts:   []string{"127.0.0.1"},
			target:  "127.0.0.1",
			wantLog: `"host":"127.0.0.1","port":"%s","decision":"allow","enforced":true}`,
		},
		{
			name:    "monitor mode tunnels a denied host and logs deny unenforced",
			guard:   allowLocal,
			monitor: true,
			target:  "127.0.0.1",
			wantLog: `"host":"127.0.0.1","port":"%s","decision":"deny","enforced":false}`,
		},
		{
			name:       "exact entry can reach a private address",
			guard:      privateOnlyExact,
			hosts:      []string{"localhost"},
			target:     "localhost",
			serverName: "localhost",
			wantLog:    `"host":"localhost","port":"%s","decision":"allow","enforced":true}`,
		},
		{
			name:       "monitor mode tunnels another tls server name and logs the mismatch unenforced",
			guard:      allowLocal,
			hosts:      []string{"127.0.0.1"},
			monitor:    true,
			target:     "127.0.0.1",
			serverName: "other.example.test",
			wantLog:    `"decision":"deny","enforced":false,"reason":"sni-mismatch"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			_, upstreamPort := startEchoServer(t)
			stubDefaultPorts(t, upstreamPort)
			var logBuf syncBuffer
			proxy := startProxy(t, tc.guard(NewServer(NewAllowlist(hostRules(tc.hosts...)), NewLogger(&logBuf), tc.monitor, nil)))
			hello := clientHello(t, tc.serverName)

			// Act
			conn := rawConnect(t, proxy.Listener.Addr().String(), net.JoinHostPort(tc.target, upstreamPort))
			defer conn.Close() //nolint:errcheck
			echo, err := echoed(t, conn, hello)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, hello, echo)
			wantLog := strings.ReplaceAll(tc.wantLog, "%s", upstreamPort)
			assert.Eventually(t, func() bool { return strings.Contains(logBuf.String(), wantLog) }, 5*time.Second, 10*time.Millisecond)
		})
	}

	t.Run("failed hijack returns a fixed 500 and logs the error once", func(t *testing.T) {
		// Arrange
		upstreamHost, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)
		var logBuf bytes.Buffer
		server := allowLocal(NewServer(NewAllowlist(hostRules(upstreamHost)), NewLogger(&logBuf), false, nil))
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

	t.Run("bytes sent with the connect reach the upstream", func(t *testing.T) {
		// Arrange
		upstreamHost, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)
		target := net.JoinHostPort(upstreamHost, upstreamPort)

		proxy := startProxy(t, allowLocal(NewServer(NewAllowlist(hostRules(upstreamHost)), NewLogger(nil), false, nil)))

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
		proxy := startProxy(t, allowLocal(NewServer(NewAllowlist(hostRules(upstreamHost)), NewLogger(&logBuf), false, nil)))

		// Act
		resp := connect(t, proxy.Listener.Addr().String(), net.JoinHostPort(upstreamHost, upstreamPort))
		defer resp.Body.Close() //nolint:errcheck
		body, _ := io.ReadAll(resp.Body)

		// Assert
		assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
		assert.Equal(t, "upstream unreachable via agentic proxy\n", string(body))
		assert.Contains(t, logBuf.String(), `"reason":"upstream-failed","error":"dial tcp `)
	})

	t.Run("tls server name of another host closes the tunnel and logs the mismatch", func(t *testing.T) {
		// Arrange
		upstreamHost, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)

		logBuf := &syncBuffer{}
		proxy := startProxy(t, allowLocal(NewServer(NewAllowlist(hostRules(upstreamHost)), NewLogger(logBuf), false, nil)))
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
		proxy := startProxy(t, allowLocal(NewServer(NewAllowlist(hostRules(upstreamHost)), NewLogger(logBuf), false, nil)))

		// Act
		conn := rawConnect(t, proxy.Listener.Addr().String(), net.JoinHostPort(upstreamHost, upstreamPort))
		defer conn.Close() //nolint:errcheck
		_, err := echoed(t, conn, []byte("GET / HTTP/1.1\r\nHost: other.example.test\r\n\r\n"))

		// Assert
		assert.ErrorIs(t, err, io.EOF)
		assert.Contains(t, logBuf.String(), `"decision":"deny","enforced":true,"reason":"sni-mismatch","error":"read tls client hello: `)
	})

	t.Run("client closing before its hello logs client-closed", func(t *testing.T) {
		// Arrange
		upstreamHost, upstreamPort := startEchoServer(t)
		stubDefaultPorts(t, upstreamPort)

		logBuf := &syncBuffer{}
		proxy := startProxy(t, allowLocal(NewServer(NewAllowlist(hostRules(upstreamHost)), NewLogger(logBuf), false, nil)))

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
		proxy := startProxy(t, NewServer(NewAllowlist(nil), NewLogger(io.Discard), false, nil))

		// Act
		resp := proxyGet(t, proxy.URL, "http://evil.test/")
		defer resp.Body.Close() //nolint:errcheck

		// Assert
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("https absolute-form is logged on port 443 with the normalized host", func(t *testing.T) {
		// Arrange
		var logBuf bytes.Buffer
		proxy := startProxy(t, NewServer(NewAllowlist(nil), NewLogger(&logBuf), false, nil))
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
		upstream := startHelloServer(t)
		upstreamHost, upstreamPort := splitHostPort(strings.TrimPrefix(upstream.URL, "http://"))
		stubDefaultPorts(t, upstreamPort)

		proxy := startProxy(t, allowLocal(NewServer(NewAllowlist(hostRules(upstreamHost)), NewLogger(io.Discard), false, nil)))

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
		proxy := startProxy(t, allowLocal(NewServer(NewAllowlist(hostRules(upstreamHost)), NewLogger(&logBuf), false, nil)))

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
		upstream := startHelloServer(t)

		var logBuf bytes.Buffer
		proxy := startProxy(t, allowLocal(NewServer(NewAllowlist(nil), NewLogger(&logBuf), true, nil)))

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
		upstream := startHelloServer(t)
		upstreamHost, upstreamPort := splitHostPort(strings.TrimPrefix(upstream.URL, "http://"))
		stubDefaultPorts(t, upstreamPort)

		var logBuf bytes.Buffer
		proxy := startProxy(t, NewServer(NewAllowlist(hostRules(upstreamHost)), NewLogger(&logBuf), false, nil))

		// Act
		resp := proxyGet(t, proxy.URL, upstream.URL)
		defer resp.Body.Close() //nolint:errcheck

		// Assert
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Contains(t, logBuf.String(), `"decision":"deny","enforced":true,"reason":"blocked-address"`)
	})
}

func Test_Server_decrypt(t *testing.T) {
	cred := testPlaceholderCredential("api.example.test", testPlaceholder)
	allow := NewAllowlist([]Rule{{Host: "Raw.Example.Test", Paths: []string{"/a"}}, {Host: "other.example.test"}})
	inject := NewInjector(newTestProxyCA(t, "api.example.test", "raw.example.test"), []Credential{cred})

	for _, tc := range []struct {
		name     string
		inject   *Injector
		host     string
		port     string
		wantOK   bool
		wantCred Credential
	}{
		{name: "credentialed host is decrypted with its credential", inject: inject, host: "api.example.test", port: "443", wantOK: true, wantCred: cred},
		{name: "path rule host is decrypted without a credential", inject: inject, host: "raw.example.test.", port: "443", wantOK: true},
		{name: "path rule host on another port is not decrypted", inject: inject, host: "raw.example.test", port: "80"},
		{name: "host allowed whole is not decrypted", inject: inject, host: "other.example.test", port: "443"},
		{name: "nothing is decrypted without an injector", host: "raw.example.test", port: "443"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			s := NewServer(allow, NewLogger(nil), false, tc.inject)

			// Act
			got, ok := s.decrypt(tc.host, tc.port)

			// Assert
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantCred, got)
		})
	}
}

func Test_Server_logClientClosed(t *testing.T) {
	// Arrange
	var logBuf bytes.Buffer
	s := NewServer(NewAllowlist(nil), NewLogger(&logBuf), false, nil)
	entry := Entry{Protocol: ProtocolHTTPS, Host: "api.example.test", Port: "443", Decision: DecisionAllow, Enforced: true}

	// Act
	s.logClientClosed(entry, errors.New("test-error"))

	// Assert
	assert.Contains(t, logBuf.String(), `"host":"api.example.test"`)
	assert.Contains(t, logBuf.String(), `"reason":"client-closed","error":"test-error"`)
}

func Test_Server_upstreamContext(t *testing.T) {
	s := NewServer(NewAllowlist(hostRules("exact.example.test", ".example.test")), NewLogger(nil), false, nil)

	cases := map[string]struct {
		host string
		want bool
	}{
		"exact host may reach private addresses":        {"exact.example.test", true},
		"wildcard host may not reach private addresses": {"sub.example.test", false},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			r := httptest.NewRequest(http.MethodGet, "http://"+c.host, nil)

			// Act
			ctx := s.upstreamContext(r, c.host)

			// Assert
			assert.Equal(t, c.want, privateOK(ctx))
		})
	}
}

func Test_Server_refuseUpstream(t *testing.T) {
	cases := map[string]struct {
		err        error
		wantStatus int
		wantReason Reason
	}{
		"blocked address is refused":   {fmt.Errorf("dial: %w", errBlockedAddr), http.StatusForbidden, ReasonBlockedAddr},
		"other error is a bad gateway": {errors.New("test-error"), http.StatusBadGateway, ReasonUpstreamFailed},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			var logBuf bytes.Buffer
			s := NewServer(NewAllowlist(nil), NewLogger(&logBuf), false, nil)
			rec := httptest.NewRecorder()

			// Act
			s.refuseUpstream(rec, Entry{Host: "api.example.test"}, c.err)

			// Assert
			assert.Equal(t, c.wantStatus, rec.Code)
			assert.Contains(t, logBuf.String(), `"reason":"`+string(c.wantReason)+`"`)
		})
	}
}

func Test_copyResponse(t *testing.T) {
	// Arrange
	resp := &http.Response{
		StatusCode: http.StatusTeapot,
		Header:     http.Header{"X-Test": {"a", "b"}},
		Body:       io.NopCloser(strings.NewReader("test-body")),
	}
	rec := httptest.NewRecorder()

	// Act
	copyResponse(rec, resp)

	// Assert
	assert.Equal(t, http.StatusTeapot, rec.Code)
	assert.Equal(t, []string{"a", "b"}, rec.Header().Values("X-Test"))
	assert.Equal(t, "test-body", rec.Body.String())
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

func Test_newHTTPServer(t *testing.T) {
	// Arrange
	handler := http.NotFoundHandler()

	// Act
	server := newHTTPServer(handler)

	// Assert
	assert.Equal(t, handshakeTimeout, server.ReadHeaderTimeout)
	assert.Equal(t, idleTimeout, server.IdleTimeout)
	assert.NotNil(t, server.Handler)
	assert.NotNil(t, server.ErrorLog)
}
