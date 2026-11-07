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

// checkServerName refuses a tunnel whose SNI isn't the CONNECT host (domain fronting), returning the bytes to replay.
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

	s.logger.Log(entry.refused(ReasonSNIMismatch, s.monitor))
	return hello, s.monitor
}

// readServerName reads the tunnel's ClientHello and returns its SNI plus the bytes read.
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

	_ = client.SetReadDeadline(time.Now().Add(handshakeTimeout))
	err = peek.Handshake()
	_ = client.SetReadDeadline(time.Time{})

	if hello == nil {
		return "", recorded.Bytes(), fmt.Errorf("read tls client hello: %w", err)
	}
	return hello.ServerName, recorded.Bytes(), nil
}

// serverNameMatches reports whether serverName is host, or empty for an IP host.
func serverNameMatches(serverName, host string) bool {
	if serverName == "" {
		_, err := netip.ParseAddr(host)
		return err == nil
	}
	return normalizeHost(serverName) == host
}
