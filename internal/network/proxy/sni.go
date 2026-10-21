package proxy

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"
)

// maxHelloBytes bounds how much of a tunnel the proxy reads looking for the ClientHello.
const maxHelloBytes = 64 << 10

// errHelloRead stops the peek's handshake once the ClientHello is parsed.
var errHelloRead = errors.New("client hello read")

// helloConn feeds a TLS server the client's bytes and drops whatever it writes back, so the peek never answers the client.
type helloConn struct {
	reader io.Reader
}

// Read reads the client's bytes.
func (c helloConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

// Write drops p, such as the alert sent when the peek aborts.
func (helloConn) Write(p []byte) (int, error) {
	return len(p), nil
}

// Close does nothing; the caller owns the real conn.
func (helloConn) Close() error { return nil }

// LocalAddr returns nil; the peek never reads it.
func (helloConn) LocalAddr() net.Addr { return nil }

// RemoteAddr returns nil; the peek never reads it.
func (helloConn) RemoteAddr() net.Addr { return nil }

// SetDeadline does nothing; the caller sets the real conn's deadline.
func (helloConn) SetDeadline(time.Time) error { return nil }

// SetReadDeadline does nothing; the caller sets the real conn's deadline.
func (helloConn) SetReadDeadline(time.Time) error { return nil }

// SetWriteDeadline does nothing, as writes are dropped.
func (helloConn) SetWriteDeadline(time.Time) error { return nil }

// checkServerName reads the tunnel's ClientHello and logs entry once, refused when its server name isn't the CONNECT
// host, so an allowed host can't front for another site on the same CDN; monitor mode logs the refusal but tunnels.
// It returns the bytes read, to replay upstream, and whether to tunnel.
func (s *Server) checkServerName(client net.Conn, entry Entry) ([]byte, bool) {
	serverName, hello, err := readServerName(client)
	switch {
	case err != nil && len(hello) == 0 && clientClosed(err):
		s.logClientClosed(entry, err)
		return nil, false
	case err != nil:
		entry.Error = err.Error()
	case !serverNameMatches(serverName, entry.Host):
		entry.Error = fmt.Sprintf("tls server name %q", serverName)
	default:
		s.logger.Log(entry)
		return hello, true
	}

	entry.Decision = DecisionDeny
	entry.Enforced = !s.monitor
	entry.Reason = ReasonSNIMismatch
	s.logger.Log(entry)
	return hello, s.monitor
}

// readServerName reads the tunnel's TLS ClientHello from client and returns its server name (SNI) plus every byte read,
// to replay upstream. It fails when the tunnel doesn't start with a ClientHello within dialTimeout.
func readServerName(client net.Conn) (serverName string, read []byte, err error) {
	var recorded bytes.Buffer
	reader := io.TeeReader(io.LimitReader(client, maxHelloBytes), &recorded)

	var hello *tls.ClientHelloInfo
	peek := tls.Server(helloConn{reader: reader}, &tls.Config{
		GetConfigForClient: func(info *tls.ClientHelloInfo) (*tls.Config, error) {
			hello = info
			return nil, errHelloRead
		},
	})

	_ = client.SetReadDeadline(time.Now().Add(dialTimeout))
	err = peek.Handshake()
	_ = client.SetReadDeadline(time.Time{})

	if hello == nil {
		return "", recorded.Bytes(), fmt.Errorf("read tls client hello: %w", err)
	}
	return hello.ServerName, recorded.Bytes(), nil
}

// serverNameMatches reports whether a tunnel to host may carry serverName: the same name, or none when host is an IP,
// which clients don't send as SNI.
func serverNameMatches(serverName, host string) bool {
	if serverName == "" {
		_, err := netip.ParseAddr(host)
		return err == nil
	}
	return normalizeHost(serverName) == host
}
