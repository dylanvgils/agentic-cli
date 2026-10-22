package proxy

import (
	"fmt"
	"net"
	"net/http"
	"slices"
)

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

// clientHandler wraps next with restrictClients for cidrs; required refuses an empty list, as injecting credentials for any reachable client would leak them across runs.
func clientHandler(next http.Handler, cidrs []string, required bool) (http.Handler, error) {
	if len(cidrs) == 0 {
		if required {
			return nil, fmt.Errorf("%s must be set when credentials are injected", EnvClientNets)
		}
		return next, nil
	}

	nets, err := parseClientNets(cidrs)
	if err != nil {
		return nil, err
	}
	return restrictClients(next, nets), nil
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
