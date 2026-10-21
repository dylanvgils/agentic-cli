package proxy

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerPaths(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.URL.Path+"|"+r.Header.Get("X-Api-Key"))
	})
	upstream := httptest.NewTLSServer(mux)
	t.Cleanup(upstream.Close)

	upstreamHost, upstreamPort := splitHostPort(upstream.Listener.Addr().String())
	stubDefaultPorts(t, upstreamPort)
	rules := []Rule{{Host: upstreamHost, Paths: []string{"/org/repo/**"}}}

	t.Run("allowed path forwards and refused path gets 403 on the same tunnel", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, nil, upstreamHost)
		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist(rules), NewLogger(&logBuf, nil, nil), false, inject))
		t.Cleanup(proxy.Close)
		client := httpsClientVia(t, proxy.URL, roots)

		// Act
		allowed, allowedErr := client.Get(upstream.URL + "/org/repo/README.md")
		refused, refusedErr := client.Get(upstream.URL + "/other?token=test-secret")

		// Assert
		require.NoError(t, allowedErr)
		defer allowed.Body.Close() //nolint:errcheck
		body, _ := io.ReadAll(allowed.Body)
		assert.Equal(t, "/org/repo/README.md|", string(body))

		require.NoError(t, refusedErr)
		defer refused.Body.Close() //nolint:errcheck
		assert.Equal(t, http.StatusForbidden, refused.StatusCode)
		assert.Equal(t, 1, strings.Count(logBuf.String(), `"intercepted":true,"path":"/other","reason":"path-blocked"`))
		assert.NotContains(t, logBuf.String(), "test-secret")
	})

	t.Run("path that upstreams may read differently gets 403", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, nil, upstreamHost)
		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist(rules), NewLogger(&logBuf, nil, nil), false, inject))
		t.Cleanup(proxy.Close)

		// Act
		resp, err := httpsClientVia(t, proxy.URL, roots).Get(upstream.URL + "/org/repo/%2e%2e/other")

		// Assert
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Contains(t, logBuf.String(), `"reason":"path-blocked"`)
	})

	t.Run("monitor mode forwards a refused path and logs it unenforced", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, nil, upstreamHost)
		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist(rules), NewLogger(&logBuf, nil, nil), true, inject))
		t.Cleanup(proxy.Close)

		// Act
		resp, err := httpsClientVia(t, proxy.URL, roots).Get(upstream.URL + "/other")

		// Assert
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, logBuf.String(), `"decision":"deny","enforced":false,"intercepted":true,"path":"/other","reason":"path-blocked"`)
	})

	t.Run("credentialed host with path rules still swaps the placeholder", func(t *testing.T) {
		// Arrange
		creds := []Credential{{Hosts: []string{upstreamHost}, Headers: []string{"X-Api-Key"}, Placeholder: testPlaceholder, Secret: "real-secret"}}
		inject, roots := newTestInjector(t, upstream, creds, upstreamHost)
		proxy := httptest.NewServer(NewServer(NewAllowlist(rules), NewLogger(nil, nil, nil), false, inject))
		t.Cleanup(proxy.Close)

		req := mustGet(t, upstream.URL+"/org/repo/x")
		req.Header.Set("X-Api-Key", testPlaceholder)

		// Act
		resp, err := httpsClientVia(t, proxy.URL, roots).Do(req)

		// Assert
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, "/org/repo/x|real-secret", string(body))
	})

	t.Run("credential host without path rules forwards any path", func(t *testing.T) {
		// Arrange
		creds := []Credential{{Hosts: []string{upstreamHost}, Headers: []string{"X-Api-Key"}, Placeholder: testPlaceholder, Secret: "real-secret"}}
		inject, roots := newTestInjector(t, upstream, creds)
		proxy := httptest.NewServer(NewServer(NewAllowlist(hostRules(upstreamHost)), NewLogger(nil, nil, nil), false, inject))
		t.Cleanup(proxy.Close)

		// Act
		resp, err := httpsClientVia(t, proxy.URL, roots).Get(upstream.URL + "/projects/group%2Frepo")

		// Assert
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("path rule host without an injector is refused", func(t *testing.T) {
		// Arrange
		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist(rules), NewLogger(&logBuf, nil, nil), false, nil))
		t.Cleanup(proxy.Close)

		// Act
		resp := connect(t, proxy.Listener.Addr().String(), upstream.Listener.Addr().String())
		defer resp.Body.Close() //nolint:errcheck

		// Assert
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Contains(t, logBuf.String(), `"reason":"path-blocked"`)
	})

	t.Run("plain http checks the path without intercepting", func(t *testing.T) {
		// Arrange
		plain := httptest.NewServer(mux)
		t.Cleanup(plain.Close)
		plainHost, plainPort := splitHostPort(plain.Listener.Addr().String())
		stubDefaultPorts(t, plainPort)
		plainRules := []Rule{{Host: plainHost, Paths: []string{"/org/repo/**"}}}
		var logBuf bytes.Buffer
		proxy := httptest.NewServer(allowLocal(NewServer(NewAllowlist(plainRules), NewLogger(&logBuf, nil, nil), false, nil)))
		t.Cleanup(proxy.Close)

		// Act
		allowed := proxyGet(t, proxy.URL, plain.URL+"/org/repo/x")
		defer allowed.Body.Close() //nolint:errcheck
		refused := proxyGet(t, proxy.URL, plain.URL+"/other")
		defer refused.Body.Close() //nolint:errcheck

		// Assert
		assert.Equal(t, http.StatusOK, allowed.StatusCode)
		assert.Equal(t, http.StatusForbidden, refused.StatusCode)
		assert.Contains(t, logBuf.String(), `"path":"/other","reason":"path-blocked"`)
		assert.NotContains(t, logBuf.String(), `"intercepted"`)
	})

	t.Run("monitor mode keeps a path verdict when the upstream then fails", func(t *testing.T) {
		// Arrange
		gone := httptest.NewServer(mux)
		goneHost, gonePort := splitHostPort(gone.Listener.Addr().String())
		gone.Close()
		stubDefaultPorts(t, gonePort)
		logPath := filepath.Join(t.TempDir(), "access.log")
		logFile, err := os.Create(logPath)
		require.NoError(t, err)
		t.Cleanup(func() { logFile.Close() }) //nolint:errcheck
		goneRules := []Rule{{Host: goneHost, Paths: []string{"/org/repo/**"}}}
		proxy := httptest.NewServer(allowLocal(NewServer(NewAllowlist(goneRules), NewLogger(logFile, nil, nil), true, nil)))
		t.Cleanup(proxy.Close)

		// Act
		resp := proxyGet(t, proxy.URL, "http://"+net.JoinHostPort(goneHost, gonePort)+"/other")
		defer resp.Body.Close() //nolint:errcheck
		summary := ReadSummary(logPath)

		// Assert
		assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
		assert.Equal(t, 1, summary.Paths.Count)
		assert.Zero(t, summary.Denied.Count)
		assert.Equal(t, 1, summary.Attempts)
	})
}

func Test_Server_checkPaths(t *testing.T) {
	rules := []Rule{
		{Host: "raw.example.test", Paths: []string{"/org/repo/**"}},
		{Host: "api.example.test"},
	}
	s := NewServer(NewAllowlist(rules), NewLogger(nil, nil, nil), false, nil)

	cases := map[string]struct {
		host string
		want string
	}{
		"path rule host drops the upgrade": {"raw.example.test", "|"},
		"host without path rules keeps it": {"api.example.test", "h2c|AAMAAABkAARAAAAAAAIAAAAA"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			var got string
			next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = r.Header.Get("Upgrade") + "|" + r.Header.Get("HTTP2-Settings")
			})
			req := httptest.NewRequest(http.MethodGet, "https://"+c.host+"/org/repo/x", nil)
			req.Header.Set("Upgrade", "h2c")
			req.Header.Set("HTTP2-Settings", "AAMAAABkAARAAAAAAAIAAAAA")

			// Act
			s.checkPaths(next, Entry{Host: c.host}).ServeHTTP(httptest.NewRecorder(), req)

			// Assert
			assert.Equal(t, c.want, got)
		})
	}
}

func TestServerPathsUnrestricted(t *testing.T) {
	// Arrange
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.URL.EscapedPath())
	}))
	t.Cleanup(plain.Close)
	plainHost, plainPort := splitHostPort(plain.Listener.Addr().String())
	stubDefaultPorts(t, plainPort)
	proxy := httptest.NewServer(allowLocal(NewServer(NewAllowlist(hostRules(plainHost)), NewLogger(nil, nil, nil), false, nil)))
	t.Cleanup(proxy.Close)

	// Act
	resp := proxyGet(t, proxy.URL, plain.URL+"/projects/group%2Frepo")
	defer resp.Body.Close() //nolint:errcheck
	body, _ := io.ReadAll(resp.Body)

	// Assert
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/projects/group%2Frepo", string(body))
}

func Test_requestPath(t *testing.T) {
	t.Run("returns the decoded path of a safe request", func(t *testing.T) {
		cases := map[string]string{
			"":                  "/",
			"/org/repo/x":       "/org/repo/x",
			"/org/re%70o/a%20b": "/org/repo/a b",
		}
		for raw, want := range cases {
			// Arrange
			u := &url.URL{Path: mustUnescape(t, raw), RawPath: raw}

			// Act
			got, ok := requestPath(u)

			// Assert
			assert.True(t, ok, raw)
			assert.Equal(t, want, got, raw)
		}
	})

	t.Run("refuses an opaque target", func(t *testing.T) {
		// Arrange
		u := &url.URL{Scheme: "http", Opaque: "secret/area"}

		// Act
		_, ok := requestPath(u)

		// Assert
		assert.False(t, ok)
	})

	t.Run("refuses paths upstreams may read differently", func(t *testing.T) {
		raws := []string{
			"/org/a%252f..%252f..%252fadmin/x",
			"/org/a%00/x",
			"/org/%c0%ae%c0%ae/x",
			"/org/%ef%bc%8e%ef%bc%8e/x",
			"/org/..%20/x",
			"/org/repo./x",
			"*",
			"/org/repo/%2e%2e/secret",
			"/org/repo/%2E%2E/secret",
			"/org/repo%2fsecret",
			"/org/repo%5csecret",
			"/org/repo\\secret",
			"/org/repo/..;/secret",
			"/org/repo%3bx",
			"/org/repo/../secret",
			"/org/./repo",
		}
		for _, raw := range raws {
			// Arrange
			u := &url.URL{Path: mustUnescape(t, raw), RawPath: raw}

			// Act
			_, ok := requestPath(u)

			// Assert
			assert.False(t, ok, raw)
		}
	})
}

func Test_dropUpgrade(t *testing.T) {
	cases := map[string]struct {
		upgrade string
		want    string
	}{
		"websocket is kept":     {"websocket", "websocket"},
		"websocket in any case": {"WebSocket", "WebSocket"},
		"h2c is dropped":        {"h2c", ""},
		"a list is dropped":     {"websocket, h2c", ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			h := http.Header{"Upgrade": {c.upgrade}, "Http2-Settings": {"AAMAAABkAARAAAAAAAIAAAAA"}}

			// Act
			dropUpgrade(h)

			// Assert
			assert.Equal(t, c.want, h.Get("Upgrade"))
		})
	}
}
