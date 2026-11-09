package proxy

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	// dialTimeout bounds how long the proxy waits to establish an upstream connection.
	dialTimeout = 30 * time.Second

	// handshakeTimeout bounds a TLS handshake, the tunnel's ClientHello read and a client request's headers.
	handshakeTimeout = 30 * time.Second

	// idleTimeout closes client connections left idle between requests.
	idleTimeout = 90 * time.Second
)

// proxyFailedText is the fixed body of a 500, so the tool never sees the proxy's internal error.
const proxyFailedText = "agentic proxy error"

// Server is a forward proxy for allowlisted hosts that logs every attempt; monitor mode logs without blocking.
type Server struct {
	allow    *Allowlist
	logger   *Logger
	monitor  bool
	inject   *Injector // nil disables credential injection
	upstream upstream
}

// NewServer builds a Server enforcing allow unless monitor; a nil inject disables injection.
func NewServer(allow *Allowlist, logger *Logger, monitor bool, inject *Injector) *Server {
	return &Server{allow: allow, logger: logger, monitor: monitor, inject: inject, upstream: newUpstream(blockedAddr)}
}

// ServeHTTP handles both CONNECT (HTTPS tunnels) and plain HTTP forwarding.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		s.handleConnect(w, r)
		return
	}
	s.handleHTTP(w, r)
}

// handleConnect checks a CONNECT and intercepts or tunnels it.
func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	host, port := splitHostPort(r.Host)
	host = normalizeHost(host)

	entry, forward := s.verdict(ProtocolHTTPS, host, port)
	if !forward {
		s.refuseHost(w, entry)
		return
	}

	// intercept logs once the client handshake succeeds
	if cred, ok := s.decrypt(host, port); ok {
		s.intercept(w, entry, cred)
		return
	}

	// A plain tunnel hides the path, so a path rule host only passes one in monitor mode
	if s.allow.pathRestricted(host) {
		blocked := s.markPathBlocked(entry)
		if blocked.Enforced {
			s.refusePath(w, blocked)
			return
		}
		s.logger.Log(blocked)
	}

	s.tunnel(w, r, entry)
}

// handleHTTP checks a plain HTTP request and forwards it.
func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	host, port := splitHostPort(r.Host)
	host = normalizeHost(host)
	if port == "" {
		port = defaultPort(r.URL.Scheme)
	}

	entry, forward := s.verdict(ProtocolHTTP, host, port)
	if !forward {
		s.refuseHost(w, entry)
		return
	}

	if !s.passPath(w, entry, r.URL) {
		return
	}

	s.forward(w, r, entry)
}

// forward sends r to entry's host and copies the response back.
func (s *Server) forward(w http.ResponseWriter, r *http.Request, entry Entry) {
	r.RequestURI = ""
	resp, err := s.upstream.transport.RoundTrip(r.WithContext(s.upstreamContext(r, entry.Host)))
	if err != nil {
		s.refuseUpstream(w, entry, err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	s.logger.Log(entry)
	copyResponse(w, resp)
}

// verdict returns the unlogged access entry and whether to forward; monitor mode always forwards.
func (s *Server) verdict(protocol Protocol, host, port string) (Entry, bool) {
	allowed := s.allow.Allows(host, port)

	decision := DecisionDeny
	if allowed {
		decision = DecisionAllow
	}

	entry := Entry{
		Protocol: protocol,
		Host:     host,
		Port:     port,
		Decision: decision,
		Enforced: !s.monitor,
	}
	return entry, allowed || s.monitor
}

// upstreamContext returns r's context, letting the dial reach private addresses only for an exact allowlist host.
func (s *Server) upstreamContext(r *http.Request, host string) context.Context {
	return withPrivateOK(r.Context(), s.allow.exactMatch(host))
}

// decrypt reports whether to terminate host's tunnel, with its credential if it has one.
func (s *Server) decrypt(host, port string) (Credential, bool) {
	if cred, ok := s.inject.credentialFor(host, port); ok {
		return cred, true
	}
	return Credential{}, s.inject.intercepts(port) && s.allow.pathRestricted(host)
}

// refuseHost logs entry and answers 403 for a host the allowlist doesn't allow.
func (s *Server) refuseHost(w http.ResponseWriter, entry Entry) {
	s.logger.Log(entry)
	http.Error(w, "host not allowed by agentic proxy allowlist", http.StatusForbidden)
}

// refuseBlocked logs entry as denied by the address guard, which monitor mode also enforces, and answers 403.
func (s *Server) refuseBlocked(w http.ResponseWriter, entry Entry) {
	s.logger.Log(entry.refused(ReasonBlockedAddr, false))
	http.Error(w, errBlockedAddr.Error(), http.StatusForbidden)
}

// refuseUpstream answers a failed upstream dial or request: 403 for a blocked address, else 502.
func (s *Server) refuseUpstream(w http.ResponseWriter, entry Entry, err error) {
	if errors.Is(err, errBlockedAddr) {
		s.refuseBlocked(w, entry)
		return
	}
	s.failUpstream(w, entry, err)
}

// failUpstream logs entry with err, which can name resolved IPs and the resolver, and answers a fixed 502 without it.
func (s *Server) failUpstream(w http.ResponseWriter, entry Entry, err error) {
	entry.Reason = ReasonUpstreamFailed
	entry.Error = err.Error()
	s.logger.Log(entry)
	http.Error(w, "upstream unreachable via agentic proxy", http.StatusBadGateway)
}

// failProxy logs entry with err, a failure of the proxy itself, and answers a fixed 500 without it.
func (s *Server) failProxy(w http.ResponseWriter, entry Entry, err error) {
	entry.Reason = ReasonProxyFailed
	entry.Error = err.Error()
	s.logger.Log(entry)
	http.Error(w, proxyFailedText, http.StatusInternalServerError)
}

// logClientClosed logs entry as closed by the client with err.
func (s *Server) logClientClosed(entry Entry, err error) {
	entry.Reason = ReasonClientClosed
	entry.Error = err.Error()
	s.logger.Log(entry)
}

// copyResponse writes resp's status, headers and body to w.
func copyResponse(w http.ResponseWriter, resp *http.Response) {
	for key, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// defaultPort returns the port for a URL without one: 443 for https, else 80.
func defaultPort(scheme string) string {
	if strings.EqualFold(scheme, "https") {
		return "443"
	}
	return "80"
}

// splitHostPort separates "host:port", returning an empty port when none is present.
func splitHostPort(hostport string) (host, port string) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport, ""
	}
	return host, port
}

// newHTTPServer returns a server for client connections, with header and idle timeouts so a client can't hold one open.
func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: handshakeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          httpErrorLog(),
	}
}
