package proxy

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// dialTimeout bounds how long the proxy waits to establish an upstream connection.
const dialTimeout = 30 * time.Second

// proxyFailedText is the fixed body of a 500, so the tool never sees the proxy's internal error.
const proxyFailedText = "agentic proxy error"

// Server is a forward proxy that permits CONNECT tunnels and plain HTTP requests only to allowlisted
// hosts, logging every attempt. In monitor mode it stops enforcing the allowlist but still logs
// the real verdict, so a run can be observed without ever blocking it.
type Server struct {
	allow    *Allowlist
	logger   *Logger
	monitor  bool
	inject   *Injector // nil disables credential injection
	upstream upstream
}

// bufferedConn reads what the HTTP server buffered past the CONNECT, such as a pipelined ClientHello, before the conn.
type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
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

// handleConnect tunnels a CONNECT request to the upstream host after checking the allowlist; denied hosts get a 403 unless in monitor mode.
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

// handleHTTP forwards a plain (non-TLS) HTTP request to the upstream host after checking the allowlist, unless in monitor mode.
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

// tunnel splices the CONNECT to entry's host once its TLS server name checks out.
func (s *Server) tunnel(w http.ResponseWriter, r *http.Request, entry Entry) {
	ctx := withPrivateOK(r.Context(), s.allow.exactMatch(entry.Host))
	upstream, err := s.upstream.dialer.DialContext(ctx, "tcp", net.JoinHostPort(entry.Host, entry.Port)) // NOSONAR - allowlisted host, address guard in the dialer
	if errors.Is(err, errBlockedAddr) {
		s.refuseBlocked(w, entry)
		return
	}
	if err != nil {
		s.failUpstream(w, entry, err)
		return
	}
	defer func() { _ = upstream.Close() }()

	client, err := hijack(w)
	if err != nil {
		s.failProxy(w, entry, err)
		return
	}
	defer func() { _ = client.Close() }()

	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		s.logger.Log(entry)
		return
	}

	hello, ok := s.checkServerName(client, entry)
	if !ok {
		return
	}
	if _, err := upstream.Write(hello); err != nil {
		return
	}

	splice(client, upstream)
}

// forward sends r to entry's host and copies the response back.
func (s *Server) forward(w http.ResponseWriter, r *http.Request, entry Entry) {
	r.RequestURI = ""
	ctx := withPrivateOK(r.Context(), s.allow.exactMatch(entry.Host))
	resp, err := s.upstream.transport.RoundTrip(r.WithContext(ctx))
	if errors.Is(err, errBlockedAddr) {
		s.refuseBlocked(w, entry)
		return
	}
	if err != nil {
		s.failUpstream(w, entry, err)
		return
	}
	s.logger.Log(entry)
	defer func() { _ = resp.Body.Close() }()

	for key, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
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
	entry.Decision = DecisionDeny
	entry.Enforced = true
	entry.Reason = ReasonBlockedAddr
	s.logger.Log(entry)
	http.Error(w, errBlockedAddr.Error(), http.StatusForbidden)
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

// Read drains the buffered bytes first, then reads the conn; the reader itself must not refill, as its source is the
// hijacked server conn.
func (c *bufferedConn) Read(p []byte) (int, error) {
	if c.reader.Buffered() > 0 {
		return c.reader.Read(p)
	}
	return c.Conn.Read(p)
}

// hijack takes over the underlying TCP connection from the ResponseWriter for a CONNECT tunnel.
func hijack(w http.ResponseWriter) (net.Conn, error) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("connection does not support hijacking")
	}

	conn, rw, err := hj.Hijack()
	if err != nil {
		return nil, fmt.Errorf("hijack: %w", err)
	}
	if rw.Reader.Buffered() > 0 {
		return &bufferedConn{Conn: conn, reader: rw.Reader}, nil
	}
	return conn, nil
}

// splice copies bytes in both directions until either side closes.
func splice(a, b net.Conn) {
	done := make(chan struct{}, 2)

	go func() {
		_, _ = io.Copy(a, b)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(b, a)
		done <- struct{}{}
	}()

	<-done
}

// defaultPort returns the port a forwarded request with scheme goes to when its URL names none: 443 for an https://
// absolute-form request, which the transport sends over TLS, else 80.
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
