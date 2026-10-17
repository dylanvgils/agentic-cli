package proxy

import (
	"bufio"
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/http/httputil"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerConnectInject(t *testing.T) {
	release := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Header.Get("X-Api-Key")+"|"+r.Host)
	})
	mux.HandleFunc("/aliases", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, strings.Join(r.Header.Values("X_Api_Key"), ","))
	})
	mux.HandleFunc("/sse", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: one\n\n")
		w.(http.Flusher).Flush()
		<-release
	})
	mux.HandleFunc("/reflect", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Echo-Api-Key", r.Header.Get("X-Api-Key"))
	})
	mux.HandleFunc("/reflect-late", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Hint", r.Header.Get("X-Api-Key"))
		w.WriteHeader(http.StatusEarlyHints)
		w.Header().Del("X-Hint")
		w.Header().Set("Trailer", "X-Echo-Api-Key")
		_, _ = io.WriteString(w, "ok")
		w.Header().Set("X-Echo-Api-Key", r.Header.Get("X-Api-Key"))
	})
	mux.HandleFunc("/private", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "real-secret" {
			w.WriteHeader(http.StatusUnauthorized)
		}
	})
	mux.HandleFunc("/drop", func(w http.ResponseWriter, _ *http.Request) {
		conn, _, _ := w.(http.Hijacker).Hijack()
		_ = conn.Close()
	})
	upstream := httptest.NewTLSServer(mux)
	t.Cleanup(upstream.Close)

	upstreamAddr := upstream.Listener.Addr().String()
	upstreamHost, upstreamPort := splitHostPort(upstreamAddr)
	stubDefaultPorts(t, upstreamPort)
	creds := []Credential{{Hosts: []string{upstreamHost}, Headers: []string{"X-Api-Key"}, Placeholder: testPlaceholder, Secret: "real-secret"}}

	t.Run("placeholder in the client's header is swapped for the secret", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, creds)
		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(&logBuf, nil, nil), false, inject))
		t.Cleanup(proxy.Close)

		req, err := http.NewRequest(http.MethodGet, upstream.URL+"/echo", nil)
		require.NoError(t, err)
		req.Header.Set("X-Api-Key", testPlaceholder)

		// Act
		resp, err := httpsClientVia(t, proxy.URL, roots).Do(req)

		// Assert
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, "real-secret|"+upstreamAddr, string(body))
		assert.Contains(t, logBuf.String(), `"intercepted":true`)
	})

	t.Run("credentialed host resolving to a blocked address returns 403 and logs the reason", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, creds)
		guarded := upstream.Client().Transport.(*http.Transport).Clone()
		guarded.DialContext = newUpstream(blockedAddr).dialer.DialContext
		inject.transport = guarded

		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(&logBuf, nil, nil), false, inject))
		t.Cleanup(proxy.Close)

		// Act
		resp, err := httpsClientVia(t, proxy.URL, roots).Get(upstream.URL + "/echo")

		// Assert
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Contains(t, logBuf.String(), `"decision":"deny","enforced":true,"intercepted":true,"reason":"blocked-address"`)
	})

	t.Run("failed upstream returns a fixed 502 and logs the error", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, creds)
		var logBuf syncBuffer
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(&logBuf, nil, nil), false, inject))
		t.Cleanup(proxy.Close)

		// Act
		resp, err := httpsClientVia(t, proxy.URL, roots).Get(upstream.URL + "/drop")

		// Assert
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
		assert.Equal(t, "upstream unreachable via agentic proxy\n", string(body))
		assert.Contains(t, logBuf.String(), `"reason":"upstream-failed","error":`)
	})

	t.Run("rejected tunnel that never sent the placeholder is logged once it closes", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, creds)
		var logBuf syncBuffer
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(&logBuf, nil, nil), false, inject))
		t.Cleanup(proxy.Close)
		client := httpsClientVia(t, proxy.URL, roots)

		// Act
		resp, err := client.Get(upstream.URL + "/private")
		require.NoError(t, err)
		_, _ = io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck
		client.CloseIdleConnections()

		// Assert
		assert.Eventually(t, func() bool {
			return strings.Contains(logBuf.String(), `"intercepted":true,"reason":"no-placeholder"`)
		}, 2*time.Second, 10*time.Millisecond)
	})

	t.Run("accepted tunnel that never sent the placeholder is not logged", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, creds)
		var logBuf syncBuffer
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(&logBuf, nil, nil), false, inject))
		t.Cleanup(proxy.Close)
		client := httpsClientVia(t, proxy.URL, roots)

		// Act
		resp, err := client.Get(upstream.URL + "/echo")
		require.NoError(t, err)
		_, _ = io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck
		client.CloseIdleConnections()

		// Assert
		assert.Never(t, func() bool {
			return strings.Contains(logBuf.String(), `"reason":"no-placeholder"`)
		}, 200*time.Millisecond, 10*time.Millisecond)
	})

	t.Run("tunnel that sent the placeholder after an unauthenticated request is not logged", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, creds)
		var logBuf syncBuffer
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(&logBuf, nil, nil), false, inject))
		t.Cleanup(proxy.Close)
		client := httpsClientVia(t, proxy.URL, roots)
		authed, err := http.NewRequest(http.MethodGet, upstream.URL+"/private", nil)
		require.NoError(t, err)
		authed.Header.Set("X-Api-Key", testPlaceholder)

		// Act
		for _, req := range []*http.Request{mustGet(t, upstream.URL+"/private"), authed} {
			resp, err := client.Do(req)
			require.NoError(t, err)
			_, _ = io.ReadAll(resp.Body)
			resp.Body.Close() //nolint:errcheck
		}
		client.CloseIdleConnections()

		// Assert
		assert.Never(t, func() bool {
			return strings.Contains(logBuf.String(), `"reason":"no-placeholder"`)
		}, 200*time.Millisecond, 10*time.Millisecond)
	})

	t.Run("secret echoed in a response header reaches the tool as the placeholder", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, creds)
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(io.Discard, nil, nil), false, inject))
		t.Cleanup(proxy.Close)
		req := mustGet(t, upstream.URL+"/reflect")
		req.Header.Set("X-Api-Key", testPlaceholder)

		// Act
		resp, err := httpsClientVia(t, proxy.URL, roots).Do(req)

		// Assert
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		assert.Equal(t, testPlaceholder, resp.Header.Get("X-Echo-Api-Key"))
	})

	t.Run("basic-encoded secret echoed in a response header reaches the tool as the placeholder", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, creds)
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(io.Discard, nil, nil), false, inject))
		t.Cleanup(proxy.Close)
		basic := "Basic " + base64.StdEncoding.EncodeToString([]byte("x-access-token:"+testPlaceholder))
		req := mustGet(t, upstream.URL+"/reflect")
		req.Header.Set("X-Api-Key", basic)

		// Act
		resp, err := httpsClientVia(t, proxy.URL, roots).Do(req)

		// Assert
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		assert.Equal(t, basic, resp.Header.Get("X-Echo-Api-Key"))
	})

	t.Run("secret echoed in a 1xx header or trailer reaches the tool as the placeholder", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, creds)
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(io.Discard, nil, nil), false, inject))
		t.Cleanup(proxy.Close)

		var hint string
		trace := &httptrace.ClientTrace{Got1xxResponse: func(_ int, header textproto.MIMEHeader) error {
			hint = header.Get("X-Hint")
			return nil
		}}
		req := mustGet(t, upstream.URL+"/reflect-late")
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
		req.Header.Set("X-Api-Key", testPlaceholder)

		// Act
		resp, err := httpsClientVia(t, proxy.URL, roots).Do(req)

		// Assert
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		_, err = io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, testPlaceholder, hint)
		assert.Equal(t, testPlaceholder, resp.Trailer.Get("X-Echo-Api-Key"))
	})

	t.Run("request host header cannot redirect the upstream", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, creds)
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(io.Discard, nil, nil), false, inject))
		t.Cleanup(proxy.Close)

		req, err := http.NewRequest(http.MethodGet, upstream.URL+"/echo", nil)
		require.NoError(t, err)
		req.Host = "evil.test"
		req.Header.Set("X-Api-Key", testPlaceholder)

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

	t.Run("underscore alias of a credential header is stripped", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, creds)
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(io.Discard, nil, nil), false, inject))
		t.Cleanup(proxy.Close)

		req, err := http.NewRequest(http.MethodGet, upstream.URL+"/aliases", nil)
		require.NoError(t, err)
		req.Header.Set("X_Api_Key", testPlaceholder)

		// Act
		resp, err := httpsClientVia(t, proxy.URL, roots).Do(req)

		// Assert
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		body, _ := io.ReadAll(resp.Body)
		assert.Empty(t, string(body))
	})

	t.Run("failed hijack returns a fixed 500 and logs the error", func(t *testing.T) {
		// Arrange
		inject, _ := newTestInjector(t, upstream, creds)
		var logBuf syncBuffer
		server := NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(&logBuf, nil, nil), false, inject)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodConnect, "http://"+upstreamAddr, nil)
		req.Host = upstreamAddr

		// Act
		server.ServeHTTP(w, req)

		// Assert
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Equal(t, proxyFailedText+"\n", w.Body.String())
		assert.Contains(t, logBuf.String(), `"reason":"proxy-failed","error":"connection does not support hijacking"`)
		assert.NotContains(t, logBuf.String(), `"intercepted"`)
	})

	t.Run("client rejecting the cert is logged as not intercepted", func(t *testing.T) {
		// Arrange
		inject, _ := newTestInjector(t, upstream, creds)
		var logBuf syncBuffer
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(&logBuf, nil, nil), false, inject))
		t.Cleanup(proxy.Close)

		// Act
		_, err := httpsClientVia(t, proxy.URL, x509.NewCertPool()).Get(upstream.URL + "/echo")

		// Assert
		require.Error(t, err)
		assert.Eventually(t, func() bool { return strings.Contains(logBuf.String(), `"reason":"proxy-failed","error":`) }, 5*time.Second, 10*time.Millisecond)
		assert.NotContains(t, logBuf.String(), `"intercepted"`)
	})

	t.Run("trace is refused before reaching the upstream", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, creds)
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(io.Discard, nil, nil), false, inject))
		t.Cleanup(proxy.Close)

		req, err := http.NewRequest(http.MethodTrace, upstream.URL+"/echo", nil)
		require.NoError(t, err)

		// Act
		resp, err := httpsClientVia(t, proxy.URL, roots).Do(req)

		// Assert
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
		assert.NotContains(t, string(body), "real-secret")
	})

	t.Run("trailing-dot host shares the normalized cert", func(t *testing.T) {
		// Arrange
		inject, _ := newTestInjector(t, upstream, creds)
		proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(io.Discard, nil, nil), false, inject))
		t.Cleanup(proxy.Close)

		// Act
		conn := rawConnect(t, proxy.Listener.Addr().String(), net.JoinHostPort(upstreamHost+".", upstreamPort))
		defer conn.Close() //nolint:errcheck

		// Assert
		assert.Len(t, inject.leaves, 1)
		assert.Contains(t, inject.leaves, upstreamHost)
	})

	t.Run("monitor mode still swaps for a denied host", func(t *testing.T) {
		// Arrange
		inject, roots := newTestInjector(t, upstream, creds)
		var logBuf bytes.Buffer
		proxy := httptest.NewServer(NewServer(NewAllowlist(nil), NewLogger(&logBuf, nil, nil), true, inject))
		t.Cleanup(proxy.Close)

		req, err := http.NewRequest(http.MethodGet, upstream.URL+"/echo", nil)
		require.NoError(t, err)
		req.Header.Set("X-Api-Key", testPlaceholder)

		// Act
		resp, err := httpsClientVia(t, proxy.URL, roots).Do(req)

		// Assert
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, "real-secret|"+upstreamAddr, string(body))
		assert.Contains(t, logBuf.String(), `"decision":"deny"`)
		assert.Contains(t, logBuf.String(), `"intercepted":true`)
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
		assert.NotContains(t, logBuf.String(), `"intercepted"`)
	})

	t.Run("host without a credential tunnels without issuing a cert", func(t *testing.T) {
		// Arrange
		echoHost, echoPort := startEchoServer(t)
		stubDefaultPorts(t, echoPort)
		other := []Credential{{Hosts: []string{"other.test"}, Headers: creds[0].Headers, Placeholder: testPlaceholder, Secret: "real-secret"}}
		inject, _ := newTestInjector(t, upstream, other)
		var logBuf bytes.Buffer
		proxy := httptest.NewServer(allowLocal(NewServer(NewAllowlist([]string{echoHost}), NewLogger(&logBuf, nil, nil), false, inject)))
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
		assert.NotContains(t, logBuf.String(), `"intercepted"`)
	})

	t.Run("credentialed host on another port tunnels without issuing a cert", func(t *testing.T) {
		// Arrange
		echoHost, echoPort := startEchoServer(t)
		stubDefaultPorts(t, echoPort)
		echoCreds := []Credential{{Hosts: []string{echoHost}, Headers: creds[0].Headers, Placeholder: testPlaceholder, Secret: "real-secret"}}
		inject, _ := newTestInjector(t, upstream, echoCreds)
		var logBuf bytes.Buffer
		proxy := httptest.NewServer(allowLocal(NewServer(NewAllowlist([]string{echoHost}), NewLogger(&logBuf, nil, nil), false, inject)))
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
		assert.NotContains(t, logBuf.String(), `"intercepted"`)
	})

	t.Run("plain http to a credentialed host is not intercepted", func(t *testing.T) {
		// Arrange
		plain := httptest.NewServer(mux)
		t.Cleanup(plain.Close)
		plainHost, plainPort := splitHostPort(plain.Listener.Addr().String())
		stubDefaultPorts(t, plainPort)
		inject, _ := newTestInjector(t, upstream, creds)
		var logBuf bytes.Buffer
		proxy := httptest.NewServer(allowLocal(NewServer(NewAllowlist([]string{plainHost}), NewLogger(&logBuf, nil, nil), false, inject)))
		t.Cleanup(proxy.Close)

		// Act
		resp := proxyGet(t, proxy.URL, plain.URL+"/echo")
		defer resp.Body.Close() //nolint:errcheck
		body, _ := io.ReadAll(resp.Body)

		// Assert
		assert.Equal(t, "|"+plain.Listener.Addr().String(), string(body))
		assert.NotContains(t, logBuf.String(), `"intercepted"`)
	})
}

func TestServerConnectSubstitute(t *testing.T) {
	placeholder := testPlaceholder
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Header.Get("Authorization")+"|"+r.Header.Get("X-Other"))
	}))
	t.Cleanup(upstream.Close)

	upstreamHost, upstreamPort := splitHostPort(upstream.Listener.Addr().String())
	stubDefaultPorts(t, upstreamPort)
	inject, roots := newTestInjector(t, upstream, []Credential{testPlaceholderCredential(upstreamHost, placeholder)})
	proxy := httptest.NewServer(NewServer(NewAllowlist([]string{upstreamHost}), NewLogger(io.Discard, nil, nil), false, inject))
	t.Cleanup(proxy.Close)

	cases := map[string]struct{ authorization, want string }{
		"bearer placeholder is swapped": {"Bearer " + placeholder, "Bearer test-secret"},
		"basic placeholder is swapped":  {"Basic " + base64.StdEncoding.EncodeToString([]byte("x-access-token:"+placeholder)), "Basic " + base64.StdEncoding.EncodeToString([]byte("x-access-token:test-secret"))},
		"the tool's own value passes":   {"Bearer own-token", "Bearer own-token"},
		"no header stays absent":        {"", ""},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			req, err := http.NewRequest(http.MethodGet, upstream.URL, nil)
			require.NoError(t, err)
			if c.authorization != "" {
				req.Header.Set("Authorization", c.authorization)
			}
			req.Header.Set("X-Other", placeholder)

			// Act
			resp, err := httpsClientVia(t, proxy.URL, roots).Do(req)

			// Assert
			require.NoError(t, err)
			defer resp.Body.Close() //nolint:errcheck
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, c.want+"|"+placeholder, string(body), "headers the credential doesn't list keep the placeholder")
		})
	}
}

func Test_credentialFor(t *testing.T) {
	cred := testPlaceholderCredential("api.example.test", testPlaceholder)
	inject := NewInjector(newTestProxyCA(t, "api.example.test"), []Credential{cred})

	t.Run("nil injector has no credential", func(t *testing.T) {
		// Arrange
		var none *Injector

		// Act
		_, ok := none.credentialFor("api.example.test", "443")

		// Assert
		assert.False(t, ok)
	})

	t.Run("exact host matches regardless of case and trailing dot", func(t *testing.T) {
		// Act
		got, ok := inject.credentialFor("API.example.test.", "443")

		// Assert
		require.True(t, ok)
		assert.Equal(t, cred, got)
	})

	t.Run("subdomain of a credentialed host has no credential", func(t *testing.T) {
		// Act
		_, ok := inject.credentialFor("evil.api.example.test", "443")

		// Assert
		assert.False(t, ok)
	})

	t.Run("unmatched host has no credential", func(t *testing.T) {
		// Act
		_, ok := inject.credentialFor("example-test.evil", "443")

		// Assert
		assert.False(t, ok)
	})

	t.Run("credentialed host on another port has no credential", func(t *testing.T) {
		for _, port := range []string{"80", "8443", ""} {
			// Act
			_, ok := inject.credentialFor("api.example.test", port)

			// Assert
			assert.False(t, ok, port)
		}
	})
}

func Test_reverseProxy(t *testing.T) {
	inject := NewInjector(newTestProxyCA(t, "api.example.test"), nil)
	cases := map[string]struct{ host, port string }{
		"api.example.test":      {"api.example.test", "443"},
		"api.example.test:8443": {"api.example.test", "8443"},
		"[::1]":                 {"::1", "443"},
		"[::1]:8443":            {"::1", "8443"},
		"127.0.0.1":             {"127.0.0.1", "443"},
	}

	for want, target := range cases {
		t.Run("targets "+want, func(t *testing.T) {
			// Arrange
			in := httptest.NewRequest(http.MethodGet, "https://evil.test/path", nil)
			pr := &httputil.ProxyRequest{In: in, Out: in.Clone(in.Context())}

			// Act
			inject.reverseProxy(target.host, target.port, Credential{}, strings.NewReplacer(), func(*http.Response) {}).Rewrite(pr)

			// Assert
			assert.Equal(t, want, pr.Out.URL.Host)
			assert.Equal(t, "/path", pr.Out.URL.Path)
		})
	}
}

func Test_leaf(t *testing.T) {
	t.Run("caches the cert per host", func(t *testing.T) {
		// Arrange
		inject := NewInjector(newTestProxyCA(t, "api.example.test"), nil)
		first, err := inject.leaf("api.example.test")
		require.NoError(t, err)

		// Act
		again, err := inject.leaf("api.example.test")

		// Assert
		require.NoError(t, err)
		assert.Same(t, first, again)
		assert.Equal(t, []string{"api.example.test"}, again.Leaf.DNSNames)
	})

	t.Run("an IP host gets an IP SAN and a CN that isn't a hostname", func(t *testing.T) {
		// Arrange
		inject := NewInjector(newTestProxyCA(t, "203.0.113.7"), nil)

		// Act
		cert, err := inject.leaf("203.0.113.7")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "agentic proxy", cert.Leaf.Subject.CommonName)
		assert.Empty(t, cert.Leaf.DNSNames)
		require.Len(t, cert.Leaf.IPAddresses, 1)
		assert.Equal(t, "203.0.113.7", cert.Leaf.IPAddresses[0].String())
	})
}

func Test_blockMethods(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := blockMethods(next)

	t.Run("header-reflecting and tunnel methods are refused in any case", func(t *testing.T) {
		for _, method := range []string{http.MethodTrace, "trace", "Track", http.MethodConnect} {
			// Arrange
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(method, "https://api.example.test/", nil)

			// Act
			handler.ServeHTTP(rec, req)

			// Assert
			assert.Equal(t, http.StatusMethodNotAllowed, rec.Code, method)
		}
	})

	t.Run("other methods are forwarded", func(t *testing.T) {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodOptions} {
			// Arrange
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(method, "https://api.example.test/", nil)

			// Act
			handler.ServeHTTP(rec, req)

			// Assert
			assert.Equal(t, http.StatusNoContent, rec.Code, method)
		}
	})
}
