package proxy

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/certs"
	"github.com/stretchr/testify/require"
)

// syncBuffer is a strings.Builder safe to read while the proxy writes to it.
type syncBuffer struct {
	mutex sync.Mutex
	buf   strings.Builder
}

// Write appends p under the lock.
func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.buf.Write(p)
}

// String returns the contents written so far.
func (b *syncBuffer) String() string {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.buf.String()
}

// stubDefaultPorts replaces DefaultPorts for a test, restoring it on cleanup.
func stubDefaultPorts(t *testing.T, ports ...string) {
	t.Helper()
	prev := DefaultPorts
	DefaultPorts = ports
	t.Cleanup(func() { DefaultPorts = prev })
}

// startEchoServer starts a TCP echo server torn down on cleanup, returning its host and port.
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

// rawConnect issues a CONNECT to proxyAddr for target and returns the tunneled connection on success.
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

// newTestInjector returns an Injector for creds whose upstream transport trusts upstream's cert, and a pool trusting its CA.
func newTestInjector(t *testing.T, upstream *httptest.Server, creds []Credential) (*Injector, *x509.CertPool) {
	t.Helper()

	var hosts []string
	for _, cred := range creds {
		hosts = append(hosts, cred.Hosts...)
	}

	ca := newTestProxyCA(t, hosts...)
	inject := NewInjector(ca, creds)
	inject.transport = upstream.Client().Transport

	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca.CertPEM()))
	return inject, roots
}

// httpsClientVia returns a client sending requests through proxyURL and trusting only roots.
func httpsClientVia(t *testing.T, proxyURL string, roots *x509.CertPool) *http.Client {
	t.Helper()

	parsed, err := url.Parse(proxyURL)
	require.NoError(t, err)

	transport := &http.Transport{Proxy: http.ProxyURL(parsed), TLSClientConfig: &tls.Config{RootCAs: roots}}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport, Timeout: 5 * time.Second}
}

// writeTestCADir writes a fresh CA scoped to api.example.test as CACertFile/CAKeyFile into a temp dir and returns it.
func writeTestCADir(t *testing.T) string {
	t.Helper()

	ca := newTestProxyCA(t, "api.example.test")
	keyPEM, err := ca.KeyPEM()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, CACertFile), ca.CertPEM(), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, CAKeyFile), keyPEM, 0o600))
	return dir
}

// writeTestFile writes content to name in a temp dir and returns its path.
func writeTestFile(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// newTestProxyCA returns a fresh CA scoped to names, failing the test on error.
func newTestProxyCA(t *testing.T, names ...string) certs.CA {
	t.Helper()
	ca, err := certs.NewScopedCA("test proxy CA", names)
	require.NoError(t, err)
	return ca
}
