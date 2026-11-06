package proxy

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"slices"
)

// AuthUser is the user name a client sends with the auth token, e.g. http://agentic:<token>@host:port.
const AuthUser = "agentic"

// restrictClients refuses requests from outside nets, so only containers on the proxy's per-run network can use it.
func restrictClients(next http.Handler, nets []*net.IPNet) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !clientAllowed(r.RemoteAddr, nets) {
			http.Error(w, "client not on the agentic proxy network", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireToken refuses requests without token in a Basic Proxy-Authorization header for AuthUser, and strips the header so
// it never reaches an upstream.
func requireToken(next http.Handler, token string) http.Handler {
	want := []byte("Basic " + base64.StdEncoding.EncodeToString([]byte(AuthUser+":"+token)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get("Proxy-Authorization"))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			w.Header().Set("Proxy-Authenticate", `Basic realm="agentic"`)
			http.Error(w, "agentic proxy needs authentication", http.StatusProxyAuthRequired)
			return
		}
		r.Header.Del("Proxy-Authorization")
		next.ServeHTTP(w, r)
	})
}

// clientHandler wraps next with restrictClients for cidrs and requireToken for token, each when set. With required, it
// refuses having neither, as injecting credentials for any client that can reach the proxy would leak them across runs.
func clientHandler(next http.Handler, cidrs []string, token string, required bool) (http.Handler, error) {
	if len(cidrs) == 0 && token == "" && required {
		return nil, fmt.Errorf("%s or %s must be set when credentials are injected", EnvClientNets, EnvAuthToken)
	}

	handler := next
	if token != "" {
		handler = requireToken(handler, token)
	}
	if len(cidrs) > 0 {
		nets, err := parseClientNets(cidrs)
		if err != nil {
			return nil, err
		}
		handler = restrictClients(handler, nets)
	}
	return handler, nil
}

// parseClientNets parses each CIDR in cidrs.
func parseClientNets(cidrs []string) ([]*net.IPNet, error) {
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid client network %q: %w", cidr, err)
		}
		nets = append(nets, ipNet)
	}
	return nets, nil
}

// clientAllowed reports whether the IP in remoteAddr ("ip:port") is inside one of nets.
func clientAllowed(remoteAddr string, nets []*net.IPNet) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return slices.ContainsFunc(nets, func(n *net.IPNet) bool { return n.Contains(ip) })
}
