package proxy

import (
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
}

// NewServer builds a Server recording to logger, enforcing allow unless monitor is true.
func NewServer(allow *Allowlist, logger *Logger, monitor bool) *Server {
	return &Server{allow: allow, logger: logger, monitor: monitor}
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

	if !s.allow.Allows(host, port) {
		s.logger.Log(ProtocolHTTPS, host, port, DecisionDeny, !s.monitor)
		if !s.monitor {
			http.Error(w, "host not allowed by agentic proxy allowlist", http.StatusForbidden)
			return
		}
	} else {
		s.logger.Log(ProtocolHTTPS, host, port, DecisionAllow, !s.monitor)
	}

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

// handleHTTP forwards a plain (non-TLS) HTTP request to the upstream host after checking the allowlist, unless in monitor mode.
func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	host, port := splitHostPort(r.Host)
	if port == "" {
		port = "80"
	}

	if !s.allow.Allows(host, port) {
		s.logger.Log(ProtocolHTTP, host, port, DecisionDeny, !s.monitor)
		if !s.monitor {
			http.Error(w, "host not allowed by agentic proxy allowlist", http.StatusForbidden)
			return
		}
	} else {
		s.logger.Log(ProtocolHTTP, host, port, DecisionAllow, !s.monitor)
	}

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
