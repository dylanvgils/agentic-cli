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

	for _, tc := range []struct {
		name       string
		nets       []string
		token      string
		required   bool
		remoteAddr string
		auth       string
		want       int
	}{
		{
			name:       "client inside a network is served",
			nets:       []string{"172.30.0.0/16"},
			required:   true,
			remoteAddr: "172.30.0.5:40000",
			want:       http.StatusTeapot,
		},
		{
			name:       "client outside every network is refused",
			nets:       []string{"172.30.0.0/16", "fd00::/64"},
			required:   true,
			remoteAddr: "172.18.0.3:40000",
			want:       http.StatusForbidden,
		},
		{
			name:       "no networks serves any client when not required",
			remoteAddr: "203.0.113.9:40000",
			want:       http.StatusTeapot,
		},
		{
			name:     "a token without networks is enough when required",
			token:    "test-secret",
			required: true,
			auth:     basicAuth(AuthUser, "test-secret"),
			want:     http.StatusTeapot,
		},
		{
			name:       "networks and token both apply",
			nets:       []string{"172.30.0.0/16"},
			token:      "test-secret",
			required:   true,
			remoteAddr: "172.30.0.5:40000",
			want:       http.StatusProxyAuthRequired,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			handler, err := clientHandler(next, tc.nets, tc.token, tc.required)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodConnect, "http://api.example.test:443", nil)
			if tc.remoteAddr != "" {
				req.RemoteAddr = tc.remoteAddr
			}
			if tc.auth != "" {
				req.Header.Set("Proxy-Authorization", tc.auth)
			}
			rec := httptest.NewRecorder()

			// Act
			handler.ServeHTTP(rec, req)

			// Assert
			assert.Equal(t, tc.want, rec.Code)
		})
	}

	t.Run("a client with the token in its proxy url is served", func(t *testing.T) {
		// Arrange
		handler, err := clientHandler(next, nil, "test-secret", true)
		require.NoError(t, err)
		server := startProxy(t, handler)
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

	t.Run("no networks or token is an error when required", func(t *testing.T) {
		// Act
		_, err := clientHandler(next, nil, "", true)

		// Assert
		assert.ErrorContains(t, err, EnvClientNets)
		assert.ErrorContains(t, err, EnvAuthToken)
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

	for _, tc := range []struct {
		name string
		auth string
	}{
		{name: "missing header"},
		{name: "wrong token", auth: basicAuth(AuthUser, "other-secret")},
		{name: "wrong user", auth: basicAuth("someone", "test-secret")},
	} {
		t.Run(tc.name+" is refused with a challenge", func(t *testing.T) {
			// Arrange
			seen = nil
			req := httptest.NewRequest(http.MethodConnect, "http://api.example.test:443", nil)
			if tc.auth != "" {
				req.Header.Set("Proxy-Authorization", tc.auth)
			}
			rec := httptest.NewRecorder()

			// Act
			handler.ServeHTTP(rec, req)

			// Assert
			assert.Equal(t, http.StatusProxyAuthRequired, rec.Code)
			assert.Equal(t, `Basic realm="agentic"`, rec.Header().Get("Proxy-Authenticate"))
			assert.Nil(t, seen)
		})
	}
}

func Test_clientAllowed(t *testing.T) {
	nets, err := parseClientNets([]string{"172.30.0.0/16", "fd00::/64"})
	require.NoError(t, err)

	for _, tc := range []struct {
		name string
		addr string
		want bool
	}{
		{name: "ipv4 inside", addr: "172.30.1.2:1234", want: true},
		{name: "ipv6 inside", addr: "[fd00::5]:1234", want: true},
		{name: "ipv4 outside", addr: "172.31.0.2:1234"},
		{name: "unparsable address is refused", addr: "not-an-addr"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			allowed := clientAllowed(tc.addr, nets)

			// Assert
			assert.Equal(t, tc.want, allowed)
		})
	}
}
