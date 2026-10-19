package proxy

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_readServerName(t *testing.T) {
	t.Run("returns the server name and the bytes read", func(t *testing.T) {
		// Arrange
		hello := clientHello(t, "api.example.test")
		client, peer := net.Pipe()
		t.Cleanup(func() { _ = peer.Close() })
		go func() { _, _ = peer.Write(hello) }()

		// Act
		serverName, read, err := readServerName(client)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "api.example.test", serverName)
		assert.Equal(t, hello, read)
	})

	t.Run("fails on bytes that aren't tls", func(t *testing.T) {
		// Arrange
		client, peer := net.Pipe()
		t.Cleanup(func() { _ = peer.Close() })
		go func() { _, _ = peer.Write([]byte("GET / HTTP/1.1\r\n\r\n")) }()

		// Act
		_, _, err := readServerName(client)

		// Assert
		assert.ErrorContains(t, err, "read tls client hello")
	})
}

func Test_serverNameMatches(t *testing.T) {
	cases := map[string]struct {
		serverName string
		host       string
		want       bool
	}{
		"same name":                   {"api.example.test", "api.example.test", true},
		"other case and trailing dot": {"API.example.test.", "api.example.test", true},
		"other name":                  {"other.example.test", "api.example.test", false},
		"subdomain of the host":       {"sub.api.example.test", "api.example.test", false},
		"no name for an ip":           {"", "192.0.2.1", true},
		"no name for an ipv6 address": {"", "2001:db8::1", true},
		"no name for a hostname":      {"", "api.example.test", false},
		"a name for an ip":            {"other.example.test", "192.0.2.1", false},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			got := serverNameMatches(c.serverName, c.host)

			// Assert
			assert.Equal(t, c.want, got)
		})
	}
}
