package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/certs"
)

// idleTimeout closes intercepted client connections left idle between requests.
const idleTimeout = 90 * time.Second

// blockedMethods are refused inside an intercepted tunnel: TRACE/TRACK echo the injected headers back, CONNECT would re-tunnel.
var blockedMethods = []string{http.MethodConnect, http.MethodTrace, "TRACK"}

// Injector terminates TLS for credentialed hosts and sets their headers before forwarding.
type Injector struct {
	ca        certs.CA
	rules     map[string][]InjectRule // keyed by normalized exact host
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

// NewInjector builds an Injector issuing leaves from ca for the hosts in creds.
func NewInjector(ca certs.CA, creds []Credential) *Injector {
	rules := make(map[string][]InjectRule)
	for _, cred := range creds {
		for _, host := range cred.Hosts {
			rules[normalizeHost(host)] = cred.Rules
		}
	}

	return &Injector{
		ca:     ca,
		rules:  rules,
		leaves: make(map[string]*tls.Certificate),
		transport: &http.Transport{
			DialContext:         (&net.Dialer{Timeout: dialTimeout}).DialContext,
			TLSHandshakeTimeout: dialTimeout,
			ForceAttemptHTTP2:   true,
		},
	}
}

// rulesFor returns the rules for exactly host; safe on a nil Injector.
func (i *Injector) rulesFor(host string) []InjectRule {
	if i == nil {
		return nil
	}
	return i.rules[normalizeHost(host)]
}

// leaf returns the cached server cert for host, already normalized by the caller, issuing it on first use.
func (i *Injector) leaf(host string) (*tls.Certificate, error) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	if cert, ok := i.leaves[host]; ok {
		return cert, nil
	}

	pair, err := i.ca.IssueLeaf(host, x509.ExtKeyUsageServerAuth, []string{host})
	if err != nil {
		return nil, fmt.Errorf("issue leaf for %s: %w", host, err)
	}
	cert := pair.TLSCertificate()
	i.leaves[host] = &cert
	return &cert, nil
}

// handshake hijacks the CONNECT and completes TLS as host, answering with an error if it fails before the hijack.
func (i *Injector) handshake(w http.ResponseWriter, host string) (*tls.Conn, error) {
	leaf, err := i.leaf(host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, err
	}

	client, err := hijack(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

// serve proxies requests on the client's TLS conn to host:port until it closes.
func (i *Injector) serve(tlsConn *tls.Conn, host, port string, rules []InjectRule) {
	server := &http.Server{
		Handler:           blockMethods(i.reverseProxy(host, port, rules)),
		ReadHeaderTimeout: dialTimeout,
		IdleTimeout:       idleTimeout,
	}
	_ = server.Serve(newOneConnListener(tlsConn))
}

// reverseProxy forwards to host:port, never the request's Host, with rules applied.
func (i *Injector) reverseProxy(host, port string, rules []InjectRule) *httputil.ReverseProxy {
	target := &url.URL{Scheme: "https", Host: host}
	if port != "443" {
		target.Host = net.JoinHostPort(host, port)
	}

	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			for _, rule := range rules {
				dropAliases(pr.Out.Header, rule.Header)
				pr.Out.Header.Set(rule.Header, rule.Value)
			}
		},
		Transport:     i.transport,
		FlushInterval: -1, // stream SSE
	}
}

// intercept terminates the CONNECT's TLS and serves it with rules, logging entry as injected only after the client handshake succeeds.
func (s *Server) intercept(w http.ResponseWriter, entry Entry, rules []InjectRule) {
	tlsConn, err := s.inject.handshake(w, entry.Host)
	entry.Injected = err == nil
	s.logger.Log(entry)
	if err != nil {
		return
	}
	defer func() { _ = tlsConn.Close() }()

	s.inject.serve(tlsConn, entry.Host, entry.Port, rules)
}

// blockMethods answers blockedMethods with 405 instead of forwarding them.
func blockMethods(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if slices.Contains(blockedMethods, r.Method) {
			http.Error(w, "method not allowed by agentic proxy", http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// dropAliases deletes headers an upstream may read as name, e.g. X_Api_Key for X-Api-Key in CGI-style backends.
func dropAliases(header http.Header, name string) {
	want := headerAlias(name)
	for key := range header {
		if headerAlias(key) == want {
			delete(header, key)
		}
	}
}

// headerAlias folds case and treats "_" as "-", the way CGI-style backends compare header names.
func headerAlias(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "_", "-")
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
