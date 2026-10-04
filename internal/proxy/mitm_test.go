package proxy

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerConnectInject(t *testing.T) {
	release := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Header.Get("X-Api-Key")+"|"+r.Host)
	})
	mux.HandleFunc("/sse", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: one\n\n")
		w.(http.Flusher).Flush()
		<-release
	})
	upstream := httptest.NewTLSServer(mux)
	t.Cleanup(upstream.Close)

	upstreamAddr := upstream.Listener.Addr().String()
	upstreamHost, upstreamPort := splitHostPort(upstreamAddr)
	stubDefaultPorts(t, upstreamPort)
	creds := []Credential{{Hosts: []string{upstreamHost}, Rules: []InjectRule{{Header: "X-Api-Key", Value: "real-secret"}}}}

	t.Run("header is injected over the client's value", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, creds)
		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(&logBuf, nil, nil), false, inject))
		t.Cleanup(proxy.Close)

		req, err := http.NewRequest(http.MethodGet, upstream.URL+"/echo", nil)
		require.NoError(t, err)
		req.Header.Set("X-Api-Key", "agentic-proxy-managed")

		// Act
		resp, err := httpsClientVia(t, proxy.URL, roots).Do(req)

		// Assert
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, "real-secret|"+upstreamAddr, string(body))
		assert.Contains(t, logBuf.String(), `"injected":true`)
	})

	t.Run("request host header cannot redirect the upstream", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, creds)
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(io.Discard, nil, nil), false, inject))
		t.Cleanup(proxy.Close)

		req, err := http.NewRequest(http.MethodGet, upstream.URL+"/echo", nil)
		require.NoError(t, err)
		req.Host = "evil.test"

		// Act
		resp, err := httpsClientVia(t, proxy.URL, roots).Do(req)

		// Assert
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, "real-secret|"+upstreamAddr, string(body))
	})

	t.Run("sse events stream before the response ends", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, creds)
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(io.Discard, nil, nil), false, inject))
		t.Cleanup(proxy.Close)
		defer close(release)

		// Act
		resp, err := httpsClientVia(t, proxy.URL, roots).Get(upstream.URL + "/sse")
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		line, err := bufio.NewReader(resp.Body).ReadString('\n')

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "data: one\n", line)
	})

	t.Run("monitor mode still injects for a denied host", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, creds)
		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist(nil), NewLogger(&logBuf, nil, nil), true, inject))
		t.Cleanup(proxy.Close)

		// Act
		resp, err := httpsClientVia(t, proxy.URL, roots).Get(upstream.URL + "/echo")

		// Assert
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, "real-secret|"+upstreamAddr, string(body))
		assert.Contains(t, logBuf.String(), `"decision":"deny"`)
		assert.Contains(t, logBuf.String(), `"injected":true`)
	})

	t.Run("denied credentialed host is refused without issuing a cert", func(t *testing.T) {
		// Arrange
		inject, _ := newTestInjector(t, upstream, creds)
		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist(nil), NewLogger(&logBuf, nil, nil), false, inject))
		t.Cleanup(proxy.Close)

		// Act
		resp := connect(t, proxy.Listener.Addr().String(), upstreamAddr)
		defer resp.Body.Close() //nolint:errcheck

		// Assert
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Empty(t, inject.leaves)
		assert.NotContains(t, logBuf.String(), `"injected"`)
	})

	t.Run("host without a credential tunnels without issuing a cert", func(t *testing.T) {
		// Arrange
		echoHost, echoPort := startEchoServer(t)
		stubDefaultPorts(t, echoPort)
		other := []Credential{{Hosts: []string{"other.test"}, Rules: creds[0].Rules}}
		inject, _ := newTestInjector(t, upstream, other)
		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{echoHost}), NewLogger(&logBuf, nil, nil), false, inject))
		t.Cleanup(proxy.Close)

		// Act
		conn := rawConnect(t, proxy.Listener.Addr().String(), net.JoinHostPort(echoHost, echoPort))
		defer conn.Close() //nolint:errcheck
		_, err := conn.Write([]byte("ping"))
		require.NoError(t, err)
		echo := make([]byte, 4)
		_, err = io.ReadFull(conn, echo)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "ping", string(echo))
		assert.Empty(t, inject.leaves)
		assert.NotContains(t, logBuf.String(), `"injected"`)
	})

	t.Run("plain http to a credentialed host is not injected", func(t *testing.T) {
		// Arrange
		plain := httptest.NewServer(mux)
		t.Cleanup(plain.Close)
		plainHost, plainPort := splitHostPort(plain.Listener.Addr().String())
		stubDefaultPorts(t, plainPort)
		inject, _ := newTestInjector(t, upstream, creds)
		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{plainHost}), NewLogger(&logBuf, nil, nil), false, inject))
		t.Cleanup(proxy.Close)

		// Act
		resp := proxyGet(t, proxy.URL, plain.URL+"/echo")
		defer resp.Body.Close() //nolint:errcheck
		body, _ := io.ReadAll(resp.Body)

		// Assert
		assert.Equal(t, "|"+plain.Listener.Addr().String(), string(body))
		assert.NotContains(t, logBuf.String(), `"injected"`)
	})
}

func Test_rulesFor(t *testing.T) {
	first := []InjectRule{{Header: "X-Api-Key", Value: "first"}}
	second := []InjectRule{{Header: "X-Api-Key", Value: "second"}}
	inject := NewInjector(newTestProxyCA(t), []Credential{
		{Hosts: []string{"api.example.test"}, Rules: first},
		{Hosts: []string{"*.example.test"}, Rules: second},
	})

	t.Run("nil injector has no rules", func(t *testing.T) {
		// Arrange
		var none *Injector

		// Act
		rules := none.rulesFor("api.example.test")

		// Assert
		assert.Nil(t, rules)
	})

	t.Run("first matching credential wins", func(t *testing.T) {
		// Act
		rules := inject.rulesFor("API.example.test")

		// Assert
		assert.Equal(t, first, rules)
	})

	t.Run("wildcard matches a subdomain", func(t *testing.T) {
		// Act
		rules := inject.rulesFor("uploads.example.test")

		// Assert
		assert.Equal(t, second, rules)
	})

	t.Run("unmatched host has no rules", func(t *testing.T) {
		// Act
		rules := inject.rulesFor("example-test.evil")

		// Assert
		assert.Nil(t, rules)
	})
}

func Test_leaf(t *testing.T) {
	// Arrange
	inject := NewInjector(newTestProxyCA(t), nil)
	first, err := inject.leaf("api.example.test")
	require.NoError(t, err)

	// Act
	again, err := inject.leaf("API.example.test")

	// Assert
	require.NoError(t, err)
	assert.Same(t, first, again)
	assert.Equal(t, []string{"api.example.test"}, again.Leaf.DNSNames)
}
