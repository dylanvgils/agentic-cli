package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_clientHandler(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })

	t.Run("client inside a network is served", func(t *testing.T) {
		// Arrange
		handler, err := clientHandler(next, []string{"172.30.0.0/16"}, true)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodConnect, "http://api.example.test:443", nil)
		req.RemoteAddr = "172.30.0.5:40000"
		rec := httptest.NewRecorder()

		// Act
		handler.ServeHTTP(rec, req)

		// Assert
		assert.Equal(t, http.StatusTeapot, rec.Code)
	})

	t.Run("client outside every network is refused", func(t *testing.T) {
		// Arrange
		handler, err := clientHandler(next, []string{"172.30.0.0/16", "fd00::/64"}, true)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodConnect, "http://api.example.test:443", nil)
		req.RemoteAddr = "172.18.0.3:40000"
		rec := httptest.NewRecorder()

		// Act
		handler.ServeHTTP(rec, req)

		// Assert
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("no networks serves any client when not required", func(t *testing.T) {
		// Arrange
		handler, err := clientHandler(next, nil, false)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "http://example.test", nil)
		req.RemoteAddr = "203.0.113.9:40000"
		rec := httptest.NewRecorder()

		// Act
		handler.ServeHTTP(rec, req)

		// Assert
		assert.Equal(t, http.StatusTeapot, rec.Code)
	})

	t.Run("no networks is an error when required", func(t *testing.T) {
		// Act
		_, err := clientHandler(next, nil, true)

		// Assert
		assert.ErrorContains(t, err, EnvClientNets)
	})

	t.Run("invalid cidr is an error", func(t *testing.T) {
		// Act
		_, err := clientHandler(next, []string{"not-a-cidr"}, true)

		// Assert
		assert.ErrorContains(t, err, "invalid client network")
	})
}

func Test_clientAllowed(t *testing.T) {
	nets, err := parseClientNets([]string{"172.30.0.0/16", "fd00::/64"})
	require.NoError(t, err)

	t.Run("ipv4 inside", func(t *testing.T) {
		// Act
		allowed := clientAllowed("172.30.1.2:1234", nets)

		// Assert
		assert.True(t, allowed)
	})

	t.Run("ipv6 inside", func(t *testing.T) {
		// Act
		allowed := clientAllowed("[fd00::5]:1234", nets)

		// Assert
		assert.True(t, allowed)
	})

	t.Run("ipv4 outside", func(t *testing.T) {
		// Act
		allowed := clientAllowed("172.31.0.2:1234", nets)

		// Assert
		assert.False(t, allowed)
	})

	t.Run("unparsable address is refused", func(t *testing.T) {
		// Act
		allowed := clientAllowed("not-an-addr", nets)

		// Assert
		assert.False(t, allowed)
	})
}
