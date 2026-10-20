package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/certs"
)

const (
	// idleTimeout closes intercepted client connections left idle between requests.
	idleTimeout = 90 * time.Second

	// leafCommonName names every leaf; OpenSSL checks a hostname-like CN against the CA's DNS constraints, which reject an IP host's CN
	leafCommonName = "agentic proxy"
)

// injectPort is the only port intercepted for injection; other ports to a credentialed host tunnel untouched.
const injectPort = "443"

// blockedMethods are refused inside an intercepted tunnel: TRACE/TRACK echo the injected headers back, CONNECT would re-tunnel.
var blockedMethods = []string{http.MethodConnect, http.MethodTrace, "TRACK"}

// Injector terminates TLS for credentialed hosts, putting their secrets in the headers before forwarding, and for path
// rule hosts, so their requests' paths can be checked.
type Injector struct {
	ca        certs.CA
	creds     map[string]Credential // keyed by normalized exact host
	pathHosts map[string]bool       // normalized hosts decrypted only to check paths
	port      string                // the only port intercepted, injectPort outside tests
	transport http.RoundTripper

	mutex  sync.Mutex
	leaves map[string]*tls.Certificate // per host, issued on first use
}

// oneConnListener serves a single conn, blocking Accept until it closes.
type oneConnListener struct {
	conns chan net.Conn
	done  chan struct{}
	once  sync.Once
	addr  net.Addr
}

// closeNotifyConn closes its listener when the conn itself is closed.
type closeNotifyConn struct {
	net.Conn
	listener *oneConnListener
}

// NewInjector builds an Injector issuing leaves from ca for the hosts in creds and pathHosts.
func NewInjector(ca certs.CA, creds []Credential, pathHosts []string) *Injector {
	byHost := make(map[string]Credential)
	for _, cred := range creds {
		for _, host := range cred.Hosts {
			byHost[normalizeHost(host)] = cred
		}
	}

	decrypted := make(map[string]bool, len(pathHosts))
	for _, host := range pathHosts {
		decrypted[normalizeHost(host)] = true
	}

	return &Injector{
		ca:        ca,
		creds:     byHost,
		pathHosts: decrypted,
		port:      injectPort,
		leaves:    make(map[string]*tls.Certificate),
		transport: &http.Transport{
			DialContext:         newUpstream(blockedAddr).dialer.DialContext,
			TLSHandshakeTimeout: dialTimeout,
			ForceAttemptHTTP2:   true,
		},
	}
}

// interceptFor reports whether to decrypt exactly host on the intercepted port, with its credential, empty for a path rule
// host without one; safe on a nil Injector.
func (i *Injector) interceptFor(host, port string) (Credential, bool) {
	if i == nil || port != i.port {
		return Credential{}, false
	}
	host = normalizeHost(host)
	if cred, ok := i.creds[host]; ok {
		return cred, true
	}
	return Credential{}, i.pathHosts[host]
}

// leaf returns the cached server cert for host, already normalized by the caller, issuing it on first use.
func (i *Injector) leaf(host string) (*tls.Certificate, error) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	if cert, ok := i.leaves[host]; ok {
		return cert, nil
	}

	pair, err := i.ca.IssueLeaf(leafCommonName, x509.ExtKeyUsageServerAuth, []string{host})
	if err != nil {
		return nil, fmt.Errorf("issue leaf for %s: %w", host, err)
	}
	cert := pair.TLSCertificate()
	i.leaves[host] = &cert
	return &cert, nil
}

// handshake hijacks the CONNECT and completes TLS as host, answering a fixed 500 if it fails before the hijack.
func (i *Injector) handshake(w http.ResponseWriter, host string) (*tls.Conn, error) {
	leaf, err := i.leaf(host)
	if err != nil {
		http.Error(w, proxyFailedText, http.StatusInternalServerError)
		return nil, err
	}

	client, err := hijack(w)
	if err != nil {
		http.Error(w, proxyFailedText, http.StatusInternalServerError)
		return nil, err
	}

	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		_ = client.Close()
		return nil, err
	}

	tlsConn := tls.Server(client, &tls.Config{
		Certificates: []tls.Certificate{*leaf},
		NextProtos:   []string{"http/1.1"},
		MinVersion:   tls.VersionTLS13,
	})

	_ = client.SetDeadline(time.Now().Add(dialTimeout))
	if err := tlsConn.Handshake(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("client handshake for %s: %w", host, err)
	}
	_ = client.SetDeadline(time.Time{})
	return tlsConn, nil
}

// serve answers requests on the client's TLS conn with handler until it closes.
func (i *Injector) serve(tlsConn *tls.Conn, handler http.Handler) {
	server := &http.Server{
		Handler:           blockMethods(handler),
		ReadHeaderTimeout: dialTimeout,
		IdleTimeout:       idleTimeout,
	}
	_ = server.Serve(newOneConnListener(tlsConn))
}

// reverseProxy forwards to host:port, never the request's Host, with cred's secret put in the request headers and taken
// back out of the response headers by scrub; onResponse sees each upstream response before the scrub.
func (i *Injector) reverseProxy(host, port string, cred Credential, scrub *strings.Replacer, onResponse func(*http.Response)) *httputil.ReverseProxy {
	// JoinHostPort brackets IPv6; the default port is then dropped to keep the Host header clean
	target := &url.URL{Scheme: "https", Host: strings.TrimSuffix(net.JoinHostPort(host, port), ":443")}

	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			substitute(pr.Out.Header, cred)
		},
		ModifyResponse: func(resp *http.Response) error {
			onResponse(resp)
			scrubHeader(resp.Header, scrub)
			return nil
		},
		Transport:     i.transport,
		FlushInterval: -1, // stream SSE
	}
}

// intercept terminates the CONNECT's TLS and serves it with cred, checking each request's path, logging entry as
// intercepted only after the client handshake succeeds. A credential tunnel the upstream rejected (401/403) and whose
// requests never carried the placeholder is logged again once it closes, as the tool then likely ran unauthenticated; per
// tunnel, so git's unauthenticated first request before its authed retry doesn't count, and neither does anonymous traffic
// the upstream accepted.
func (s *Server) intercept(w http.ResponseWriter, entry Entry, cred Credential) {
	tlsConn, err := s.inject.handshake(w, entry.Host)
	if clientClosed(err) {
		s.logClientClosed(entry, err)
		return
	}
	if err != nil {
		entry.Reason = ReasonProxyFailed
		entry.Error = err.Error()
		s.logger.Log(entry)
		return
	}
	entry.Intercepted = true
	s.logger.Log(entry)
	defer func() { _ = tlsConn.Close() }()

	failed := func(w http.ResponseWriter, r *http.Request, err error) {
		// The client cancelled the request or hung up, so there's no one left to answer
		if r.Context().Err() != nil {
			s.logClientClosed(entry, err)
			return
		}
		if errors.Is(err, errBlockedAddr) {
			s.refuseBlocked(w, entry)
			return
		}
		s.failUpstream(w, entry, err)
	}

	var sawAuthFailure, sawPlaceholder atomic.Bool
	onResponse := func(resp *http.Response) {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			sawAuthFailure.Store(true)
		}
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hasPlaceholder(r.Header, cred) {
			sawPlaceholder.Store(true)
		}

		// Per request, as the Basic forms to scrub depend on what this request sent
		scrub := newScrubber(r.Header, cred)
		reverse := s.inject.reverseProxy(entry.Host, entry.Port, cred, scrub, onResponse)
		reverse.ErrorHandler = failed
		reverse.ServeHTTP(&scrubWriter{ResponseWriter: w, scrub: scrub}, r)

		// Trailers land in w's header after the body and are sent once the handler returns
		scrubHeader(w.Header(), scrub)
	})
	s.inject.serve(tlsConn, s.checkPaths(allowPrivate(handler, s.allow.exactMatch(entry.Host)), entry))

	if len(cred.Headers) > 0 && sawAuthFailure.Load() && !sawPlaceholder.Load() {
		entry.Reason = ReasonNoPlaceholder
		s.logger.Log(entry)
	}
}

// blockMethods answers blockedMethods in any case with 405 instead of forwarding them, as some upstreams ignore method case.
func blockMethods(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if slices.ContainsFunc(blockedMethods, func(m string) bool { return strings.EqualFold(m, r.Method) }) {
			http.Error(w, "method not allowed by agentic proxy", http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientClosed reports whether a conn error outside a request means the client hung up.
func clientClosed(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE)
}

// newOneConnListener returns a listener yielding conn once.
func newOneConnListener(conn net.Conn) *oneConnListener {
	l := &oneConnListener{conns: make(chan net.Conn, 1), done: make(chan struct{}), addr: conn.LocalAddr()}
	l.conns <- &closeNotifyConn{Conn: conn, listener: l}
	return l
}

// Accept returns the conn on the first call, then blocks until the listener closes.
func (l *oneConnListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

// Close unblocks Accept; safe to call more than once.
func (l *oneConnListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return nil
}

// Addr returns the wrapped conn's local address.
func (l *oneConnListener) Addr() net.Addr {
	return l.addr
}

// Close closes the conn and its listener.
func (c *closeNotifyConn) Close() error {
	err := c.Conn.Close()
	_ = c.listener.Close()
	return err
}
