package proxy

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// dialTimeout bounds how long the proxy waits to establish an upstream
// connection before giving up.
const dialTimeout = 30 * time.Second

// Server is a fail-closed forward proxy. It permits HTTP CONNECT tunnels and
// plain HTTP requests only to hosts on the allowlist, logging every attempt.
type Server struct {
	allow  *Allowlist
	logger *Logger
}

// NewServer builds a Server enforcing allow and recording to logger.
func NewServer(allow *Allowlist, logger *Logger) *Server {
	return &Server{allow: allow, logger: logger}
}

// ServeHTTP handles both CONNECT (HTTPS tunnels) and plain HTTP forwarding.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		s.handleConnect(w, r)
		return
	}
	s.handleHTTP(w, r)
}

// handleConnect tunnels a CONNECT request to the upstream host after checking
// the allowlist. Denied hosts get a 403 so the tool sees an informative error.
func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	host, port := splitHostPort(r.Host)

	if !s.allow.Allows(host, port) {
		s.logger.Log(ProtocolHTTPS, host, port, DecisionDeny)
		http.Error(w, "host not allowed by agentic proxy allowlist", http.StatusForbidden)
		return
	}
	s.logger.Log(ProtocolHTTPS, host, port, DecisionAllow)

	upstream, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), dialTimeout)
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

// handleHTTP forwards a plain (non-TLS) HTTP request to the upstream host after
// checking the allowlist.
func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	host, port := splitHostPort(r.Host)
	if port == "" {
		port = "80"
	}

	if !s.allow.Allows(host, port) {
		s.logger.Log(ProtocolHTTP, host, port, DecisionDeny)
		http.Error(w, "host not allowed by agentic proxy allowlist", http.StatusForbidden)
		return
	}
	s.logger.Log(ProtocolHTTP, host, port, DecisionAllow)

	r.RequestURI = ""
	resp, err := http.DefaultTransport.RoundTrip(r)
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

// hijack takes over the underlying TCP connection from the ResponseWriter so the
// proxy can splice raw bytes for a CONNECT tunnel.
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

// splitHostPort separates "host:port"; when no port is present the port is
// returned empty.
func splitHostPort(hostport string) (host, port string) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport, ""
	}
	return host, port
}
