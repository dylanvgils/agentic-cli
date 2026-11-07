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
	"sync/atomic"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/network/certs"
)

const (
	// idleTimeout closes intercepted client connections left idle between requests.
	idleTimeout = 90 * time.Second

	// leafCommonName names every leaf; OpenSSL checks a hostname-like CN against the CA's DNS constraints, which reject an IP host's CN
	leafCommonName = "agentic proxy"
)

// injectPort is the only port intercepted for injection; other ports to a credentialed host tunnel untouched.
const injectPort = "443"

// blockedMethods are refused in an intercepted tunnel: TRACE/TRACK echo the secret, CONNECT would re-tunnel.
var blockedMethods = []string{http.MethodConnect, http.MethodTrace, "TRACK"}

// Injector terminates TLS for the tunnels the Server decrypts, putting credentialed hosts' secrets in the headers.
type Injector struct {
	ca        certs.CA
	creds     map[string]Credential // keyed by normalized exact host
	port      string                // the only port intercepted, injectPort outside tests
	transport http.RoundTripper

	mutex  sync.Mutex
	leaves map[string]*tls.Certificate // per host, issued on first use
}

// authWatch tracks a tunnel's auth failures and placeholders, per tunnel so git's anonymous first try doesn't count.
type authWatch struct {
	sawAuthFailure atomic.Bool
	sawPlaceholder atomic.Bool
}

// NewInjector builds an Injector holding creds and issuing leaves from ca.
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
		port:   injectPort,
		leaves: make(map[string]*tls.Certificate),
		transport: &http.Transport{
			DialContext:         newGuardedDialer(blockedAddr).DialContext,
			TLSHandshakeTimeout: handshakeTimeout,
			ForceAttemptHTTP2:   true,
		},
	}
}

// credentialFor returns the credential for exactly host on the intercepted port; safe on a nil Injector.
func (i *Injector) credentialFor(host, port string) (Credential, bool) {
	if !i.intercepts(port) {
		return Credential{}, false
	}
	cred, ok := i.creds[normalizeHost(host)]
	return cred, ok
}

// intercepts reports whether tunnels to port can be decrypted; safe on a nil Injector.
func (i *Injector) intercepts(port string) bool {
	return i != nil && port == i.port
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

	if _, err := io.WriteString(client, connectEstablished); err != nil {
		_ = client.Close()
		return nil, err
	}

	tlsConn := tls.Server(client, &tls.Config{
		Certificates: []tls.Certificate{*leaf},
		NextProtos:   []string{"http/1.1"},
		MinVersion:   tls.VersionTLS13,
	})

	_ = client.SetDeadline(time.Now().Add(handshakeTimeout))
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
		ReadHeaderTimeout: handshakeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          httpErrorLog(),
	}
	_ = server.Serve(newOneConnListener(tlsConn))
}

// reverseProxy forwards to host:port with cred's secret swapped in; onResponse sees each response before the scrub.
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
		ErrorLog:      httpErrorLog(),
	}
}

// intercept terminates the CONNECT's TLS and serves its requests with cred, checking their paths.
func (s *Server) intercept(w http.ResponseWriter, entry Entry, cred Credential) {
	tlsConn, ok := s.startIntercept(w, entry)
	if !ok {
		return
	}
	defer func() { _ = tlsConn.Close() }()

	entry.Intercepted = true
	s.logger.Log(entry)
	watch := &authWatch{}

	// Requests pass blockMethods (in serve), checkPaths, then allowPrivate
	handler := allowPrivate(s.injectHandler(entry, cred, watch), s.allow.exactMatch(entry.Host))
	handler = s.checkPaths(handler, entry)
	s.inject.serve(tlsConn, handler)

	if watch.missedPlaceholder(cred) {
		entry.Reason = ReasonNoPlaceholder
		s.logger.Log(entry)
	}
}

// startIntercept completes the client's TLS handshake, logging a failure.
func (s *Server) startIntercept(w http.ResponseWriter, entry Entry) (*tls.Conn, bool) {
	tlsConn, err := s.inject.handshake(w, entry.Host)
	if clientClosed(err) {
		s.logClientClosed(entry, err)
		return nil, false
	}
	if err != nil {
		entry.Reason = ReasonProxyFailed
		entry.Error = err.Error()
		s.logger.Log(entry)
		return nil, false
	}

	return tlsConn, true
}

// injectHandler forwards each decrypted request with cred swapped in and scrubbed back out of the response.
func (s *Server) injectHandler(entry Entry, cred Credential, watch *authWatch) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		watch.request(r.Header, cred)

		// Per request, as the Basic forms to scrub depend on what this request sent
		scrub := newScrubber(r.Header, cred)
		reverse := s.inject.reverseProxy(entry.Host, entry.Port, cred, scrub, watch.response)
		reverse.ErrorHandler = s.interceptFailed(entry)
		reverse.ServeHTTP(&scrubWriter{ResponseWriter: w, scrub: scrub}, r)

		// Trailers land in w's header after the body and are sent once the handler returns
		scrubHeader(w.Header(), scrub)
	})
}

// interceptFailed returns the reverse proxy's error handler for entry's tunnel.
func (s *Server) interceptFailed(entry Entry) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		// The client cancelled or hung up, so there's no one left to answer
		if r.Context().Err() != nil {
			s.logClientClosed(entry, err)
			return
		}
		s.refuseUpstream(w, entry, err)
	}
}

// request notes whether a request carried cred's placeholder.
func (a *authWatch) request(header http.Header, cred Credential) {
	if hasPlaceholder(header, cred) {
		a.sawPlaceholder.Store(true)
	}
}

// response notes an upstream 401 or 403.
func (a *authWatch) response(resp *http.Response) {
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		a.sawAuthFailure.Store(true)
	}
}

// missedPlaceholder reports an auth failure on a credential tunnel that never sent the placeholder.
func (a *authWatch) missedPlaceholder(cred Credential) bool {
	return len(cred.Headers) > 0 && a.sawAuthFailure.Load() && !a.sawPlaceholder.Load()
}

// blockMethods answers blockedMethods with 405, in any case, as some upstreams ignore it.
func blockMethods(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if slices.ContainsFunc(blockedMethods, func(m string) bool { return strings.EqualFold(m, r.Method) }) {
			http.Error(w, "method not allowed by agentic proxy", http.StatusMethodNotAllowed)
			return
		}
		next.ServeHTTP(w, r)
	})
}
