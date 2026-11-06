package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_clientHandler(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })

	t.Run("client inside a network is served", func(t *testing.T) {
		// Arrange
		handler, err := clientHandler(next, []string{"172.30.0.0/16"}, "", true)
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
		handler, err := clientHandler(next, []string{"172.30.0.0/16", "fd00::/64"}, "", true)
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
		handler, err := clientHandler(next, nil, "", false)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, "http://example.test", nil)
		req.RemoteAddr = "203.0.113.9:40000"
		rec := httptest.NewRecorder()

		// Act
		handler.ServeHTTP(rec, req)

		// Assert
		assert.Equal(t, http.StatusTeapot, rec.Code)
	})

	t.Run("no networks or token is an error when required", func(t *testing.T) {
		// Act
		_, err := clientHandler(next, nil, "", true)

		// Assert
		assert.ErrorContains(t, err, EnvClientNets)
		assert.ErrorContains(t, err, EnvAuthToken)
	})

	t.Run("a token without networks is enough when required", func(t *testing.T) {
		// Arrange
		handler, err := clientHandler(next, nil, "test-secret", true)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodConnect, "http://api.example.test:443", nil)
		req.Header.Set("Proxy-Authorization", basicAuth(AuthUser, "test-secret"))
		rec := httptest.NewRecorder()

		// Act
		handler.ServeHTTP(rec, req)

		// Assert
		assert.Equal(t, http.StatusTeapot, rec.Code)
	})

	t.Run("a client with the token in its proxy url is served", func(t *testing.T) {
		// Arrange
		handler, err := clientHandler(next, nil, "test-secret", true)
		require.NoError(t, err)
		server := httptest.NewServer(handler)
		t.Cleanup(server.Close)
		proxyURL, err := url.Parse(server.URL)
		require.NoError(t, err)
		proxyURL.User = url.UserPassword(AuthUser, "test-secret")
		client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

		// Act
		resp, err := client.Get("http://api.example.test/")

		// Assert
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		assert.Equal(t, http.StatusTeapot, resp.StatusCode)
	})

	t.Run("networks and token both apply", func(t *testing.T) {
		// Arrange
		handler, err := clientHandler(next, []string{"172.30.0.0/16"}, "test-secret", true)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodConnect, "http://api.example.test:443", nil)
		req.RemoteAddr = "172.30.0.5:40000"
		rec := httptest.NewRecorder()

		// Act
		handler.ServeHTTP(rec, req)

		// Assert
		assert.Equal(t, http.StatusProxyAuthRequired, rec.Code)
	})

	t.Run("invalid cidr is an error", func(t *testing.T) {
		// Act
		_, err := clientHandler(next, []string{"not-a-cidr"}, "", true)

		// Assert
		assert.ErrorContains(t, err, "invalid client network")
	})
}

func Test_requireToken(t *testing.T) {
	var seen *http.Request
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r
		w.WriteHeader(http.StatusTeapot)
	})
	handler := requireToken(next, "test-secret")

	t.Run("matching token is served without the header", func(t *testing.T) {
		// Arrange
		seen = nil
		req := httptest.NewRequest(http.MethodGet, "http://example.test", nil)
		req.Header.Set("Proxy-Authorization", basicAuth(AuthUser, "test-secret"))
		rec := httptest.NewRecorder()

		// Act
		handler.ServeHTTP(rec, req)

		// Assert
		assert.Equal(t, http.StatusTeapot, rec.Code)
		require.NotNil(t, seen)
		assert.Empty(t, seen.Header.Get("Proxy-Authorization"))
	})

	t.Run("missing header is refused with a challenge", func(t *testing.T) {
		// Arrange
		seen = nil
		req := httptest.NewRequest(http.MethodConnect, "http://api.example.test:443", nil)
		rec := httptest.NewRecorder()

		// Act
		handler.ServeHTTP(rec, req)

		// Assert
		assert.Equal(t, http.StatusProxyAuthRequired, rec.Code)
		assert.Equal(t, `Basic realm="agentic"`, rec.Header().Get("Proxy-Authenticate"))
		assert.Nil(t, seen)
	})

	t.Run("wrong token is refused", func(t *testing.T) {
		// Arrange
		seen = nil
		req := httptest.NewRequest(http.MethodConnect, "http://api.example.test:443", nil)
		req.Header.Set("Proxy-Authorization", basicAuth(AuthUser, "other-secret"))
		rec := httptest.NewRecorder()

		// Act
		handler.ServeHTTP(rec, req)

		// Assert
		assert.Equal(t, http.StatusProxyAuthRequired, rec.Code)
		assert.Nil(t, seen)
	})

	t.Run("wrong user is refused", func(t *testing.T) {
		// Arrange
		seen = nil
		req := httptest.NewRequest(http.MethodConnect, "http://api.example.test:443", nil)
		req.Header.Set("Proxy-Authorization", basicAuth("someone", "test-secret"))
		rec := httptest.NewRecorder()

		// Act
		handler.ServeHTTP(rec, req)

		// Assert
		assert.Equal(t, http.StatusProxyAuthRequired, rec.Code)
		assert.Nil(t, seen)
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
