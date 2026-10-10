package proxy

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// dialTimeout bounds how long the proxy waits to establish an upstream connection.
const dialTimeout = 30 * time.Second

// Server is a forward proxy that permits CONNECT tunnels and plain HTTP requests only to allowlisted
// hosts, logging every attempt. In monitor mode it stops enforcing the allowlist but still logs
// the real verdict, so a run can be observed without ever blocking it.
type Server struct {
	allow   *Allowlist
	logger  *Logger
	monitor bool
	inject  *Injector // nil disables credential injection
}

// NewServer builds a Server enforcing allow unless monitor; a nil inject disables injection.
func NewServer(allow *Allowlist, logger *Logger, monitor bool, inject *Injector) *Server {
	return &Server{allow: allow, logger: logger, monitor: monitor, inject: inject}
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
	rules := s.inject.rulesFor(host, port)

	entry, forward := s.verdict(ProtocolHTTPS, host, port)
	if !forward {
		s.logger.Log(entry)
		http.Error(w, "host not allowed by agentic proxy allowlist", http.StatusForbidden)
		return
	}

	// intercept logs once the client handshake shows whether injection happens
	if len(rules) > 0 {
		s.intercept(w, entry, rules)
		return
	}

	ctx := withPrivateOK(r.Context(), s.allow.exactMatch(host))
	upstream, err := upstreamDialer.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if errors.Is(err, errBlockedAddr) {
		s.refuseBlocked(w, entry)
		return
	}
	s.logger.Log(entry)
	if err != nil {
		http.Error(w, "upstream dial failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer func() { _ = upstream.Close() }()

	client, err := hijack(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = client.Close() }()

	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}

	splice(client, upstream)
}

// handleHTTP forwards a plain (non-TLS) HTTP request to the upstream host after checking the allowlist, unless in monitor mode.
func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	host, port := splitHostPort(r.Host)
	if port == "" {
		port = "80"
	}

	entry, forward := s.verdict(ProtocolHTTP, host, port)
	if !forward {
		s.logger.Log(entry)
		http.Error(w, "host not allowed by agentic proxy allowlist", http.StatusForbidden)
		return
	}

	r.RequestURI = ""
	ctx := withPrivateOK(r.Context(), s.allow.exactMatch(host))
	resp, err := upstreamTransport.RoundTrip(r.WithContext(ctx))
	if errors.Is(err, errBlockedAddr) {
		s.refuseBlocked(w, entry)
		return
	}
	s.logger.Log(entry)
	if err != nil {
		http.Error(w, "upstream request failed: "+err.Error(), http.StatusBadGateway)
		return
	}
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

// refuseBlocked logs entry as denied by the address guard, which monitor mode also enforces, and answers 403.
func (s *Server) refuseBlocked(w http.ResponseWriter, entry Entry) {
	entry.Decision = DecisionDeny
	entry.Enforced = true
	entry.Reason = reasonBlockedAddr
	s.logger.Log(entry)
	http.Error(w, errBlockedAddr.Error(), http.StatusForbidden)
}

// hijack takes over the underlying TCP connection from the ResponseWriter for a CONNECT tunnel.
func hijack(w http.ResponseWriter) (net.Conn, error) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("connection does not support hijacking")
	}

	conn, _, err := hj.Hijack()
	if err != nil {
		return nil, fmt.Errorf("hijack: %w", err)
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

// splitHostPort separates "host:port", returning an empty port when none is present.
func splitHostPort(hostport string) (host, port string) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport, ""
	}
	return host, port
}
