package proxy

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
)

// connectEstablished answers a CONNECT the proxy takes over, before the tunnel's first byte.
const connectEstablished = "HTTP/1.1 200 Connection Established\r\n\r\n"

// bufferedConn reads what the HTTP server buffered past the CONNECT, such as a pipelined ClientHello, before the conn.
type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

// Read drains the buffered bytes, then reads the conn.
func (c *bufferedConn) Read(p []byte) (int, error) {
	if c.reader.Buffered() > 0 {
		return c.reader.Read(p)
	}
	return c.Conn.Read(p)
}

// tunnel splices the CONNECT to entry's host once its TLS server name checks out.
func (s *Server) tunnel(w http.ResponseWriter, r *http.Request, entry Entry) {
	upstream, err := s.upstream.dialer.DialContext(s.upstreamContext(r, entry.Host), "tcp", net.JoinHostPort(entry.Host, entry.Port)) // NOSONAR - allowlisted host, address guard in the dialer
	if err != nil {
		s.refuseUpstream(w, entry, err)
		return
	}
	defer func() { _ = upstream.Close() }()

	client, ok := s.acceptTunnel(w, entry)
	if !ok {
		return
	}
	defer func() { _ = client.Close() }()

	hello, ok := s.checkServerName(client, entry)
	if !ok {
		return
	}
	if _, err := upstream.Write(hello); err != nil {
		return
	}

	splice(client, upstream)
}

// acceptTunnel hijacks the CONNECT and answers 200; on failure it logs and closes.
func (s *Server) acceptTunnel(w http.ResponseWriter, entry Entry) (net.Conn, bool) {
	client, err := hijack(w)
	if err != nil {
		s.failProxy(w, entry, err)
		return nil, false
	}

	if _, err := io.WriteString(client, connectEstablished); err != nil {
		_ = client.Close()
		s.logger.Log(entry)
		return nil, false
	}
	return client, true
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

// clientClosed reports whether a conn error outside a request means the client hung up.
func clientClosed(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE)
}
