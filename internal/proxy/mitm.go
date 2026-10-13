package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
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
var injectPort = "443"

// blockedMethods are refused inside an intercepted tunnel: TRACE/TRACK echo the injected headers back, CONNECT would re-tunnel.
var blockedMethods = []string{http.MethodConnect, http.MethodTrace, "TRACK"}

// Injector terminates TLS for credentialed hosts and puts their secrets in the headers before forwarding.
type Injector struct {
	ca        certs.CA
	creds     map[string]Credential // keyed by normalized exact host
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
	byHost := make(map[string]Credential)
	for _, cred := range creds {
		for _, host := range cred.Hosts {
			byHost[normalizeHost(host)] = cred
		}
	}

	return &Injector{
		ca:     ca,
		creds:  byHost,
		leaves: make(map[string]*tls.Certificate),
		transport: &http.Transport{
			DialContext:         upstreamDialer.DialContext,
			TLSHandshakeTimeout: dialTimeout,
			ForceAttemptHTTP2:   true,
		},
	}
}

// credentialFor returns the credential for exactly host on injectPort; safe on a nil Injector.
func (i *Injector) credentialFor(host, port string) (Credential, bool) {
	if i == nil || port != injectPort {
		return Credential{}, false
	}
	cred, ok := i.creds[normalizeHost(host)]
	return cred, ok
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

// serve answers requests on the client's TLS conn with handler until it closes.
func (i *Injector) serve(tlsConn *tls.Conn, handler http.Handler) {
	server := &http.Server{
		Handler:           blockMethods(handler),
		ReadHeaderTimeout: dialTimeout,
		IdleTimeout:       idleTimeout,
	}
	_ = server.Serve(newOneConnListener(tlsConn))
}

// reverseProxy forwards to host:port, never the request's Host, with cred's secret put in the headers.
func (i *Injector) reverseProxy(host, port string, cred Credential) *httputil.ReverseProxy {
	// JoinHostPort brackets IPv6; the default port is then dropped to keep the Host header clean
	target := &url.URL{Scheme: "https", Host: strings.TrimSuffix(net.JoinHostPort(host, port), ":443")}

	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			if cred.Placeholder != "" {
				substitute(pr.Out.Header, cred)
				return
			}
			for _, rule := range cred.Rules {
				dropAliases(pr.Out.Header, rule.Header)
				pr.Out.Header.Set(rule.Header, rule.Value)
			}
		},
		Transport:     i.transport,
		FlushInterval: -1, // stream SSE
	}
}

// intercept terminates the CONNECT's TLS and serves it with cred, logging entry as injected only after the client handshake succeeds.
func (s *Server) intercept(w http.ResponseWriter, entry Entry, cred Credential) {
	tlsConn, err := s.inject.handshake(w, entry.Host)
	entry.Injected = err == nil
	s.logger.Log(entry)
	if err != nil {
		return
	}
	defer func() { _ = tlsConn.Close() }()

	reverse := s.inject.reverseProxy(entry.Host, entry.Port, cred)
	reverse.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		if errors.Is(err, errBlockedAddr) {
			s.refuseBlocked(w, entry)
			return
		}
		http.Error(w, "upstream request failed: "+err.Error(), http.StatusBadGateway)
	}
	s.inject.serve(tlsConn, allowPrivate(reverse, s.allow.exactMatch(entry.Host)))
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

// substitute swaps cred's placeholder for its secret in each of its headers the tool sent, and drops their aliases so the
// secret reaches the upstream only under the canonical name.
func substitute(header http.Header, cred Credential) {
	for _, name := range cred.Headers {
		canonical := http.CanonicalHeaderKey(name)
		for key := range header {
			if key != canonical && headerAlias(key) == headerAlias(canonical) {
				delete(header, key)
			}
		}

		values := header[canonical]
		for i, value := range values {
			values[i] = substituteValue(value, cred.Placeholder, cred.Secret)
		}
	}
}

// substituteValue replaces placeholder with secret in value, decoding a Basic credential first, as git and curl -u send
// the placeholder base64-encoded.
func substituteValue(value, placeholder, secret string) string {
	scheme, encoded, found := strings.Cut(value, " ")
	if found && strings.EqualFold(scheme, "Basic") {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
		if err == nil && strings.Contains(string(decoded), placeholder) {
			swapped := strings.ReplaceAll(string(decoded), placeholder, secret)
			return scheme + " " + base64.StdEncoding.EncodeToString([]byte(swapped))
		}
	}
	return strings.ReplaceAll(value, placeholder, secret)
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
