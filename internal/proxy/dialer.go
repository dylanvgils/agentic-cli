package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

var (
	// errBlockedAddr marks a dial refused because the destination resolved to a local or private address.
	errBlockedAddr = errors.New("destination resolves to a blocked address")

	// alwaysBlocked holds ranges the stdlib checks miss: "this network" (reaches the proxy itself on Linux),
	// deprecated IPv4-compatible and site-local IPv6, and cloud metadata endpoints outside link-local.
	// NOSONAR on each line: ranges to refuse, not addresses to reach
	alwaysBlocked = mustParsePrefixes(
		"0.0.0.0/8",          // NOSONAR
		"::/96",              // NOSONAR
		"fec0::/10",          // NOSONAR
		"fd00:ec2::254/128",  // NOSONAR - AWS IPv6 metadata
		"100.100.100.200/32", // NOSONAR - Alibaba metadata
		"168.63.129.16/32",   // NOSONAR - Azure WireServer
	)

	// privateRanges holds private ranges IsPrivate misses: CGNAT, IETF protocol assignments and benchmarking (fake-IP VPNs).
	privateRanges = mustParsePrefixes("100.64.0.0/10", "192.0.0.0/24", "198.18.0.0/15") // NOSONAR - ranges to refuse, not addresses to reach

	// nat64Prefix and sixToFourPrefix embed an IPv4 address, which is checked instead.
	nat64Prefix     = netip.MustParsePrefix("64:ff9b::/96") // NOSONAR - a prefix to decode, not an address to reach
	sixToFourPrefix = netip.MustParsePrefix("2002::/16")    // NOSONAR - a prefix to decode, not an address to reach

	// upstreamDialer checks each resolved address before connecting, so DNS can't point an allowed name inward.
	upstreamDialer = &net.Dialer{Timeout: dialTimeout, ControlContext: guardAddr}

	// upstreamTransport forwards plain HTTP requests through upstreamDialer.
	upstreamTransport = &http.Transport{
		DialContext:         upstreamDialer.DialContext,
		TLSHandshakeTimeout: dialTimeout,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
	}
)

// privateOKKey is the context key for whether a dial may reach private addresses.
type privateOKKey struct{}

// allowPrivate passes whether next's upstream dials may reach private addresses.
func allowPrivate(next http.Handler, ok bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(withPrivateOK(r.Context(), ok)))
	})
}

// withPrivateOK returns ctx carrying whether a dial may reach private addresses.
func withPrivateOK(ctx context.Context, ok bool) context.Context {
	return context.WithValue(ctx, privateOKKey{}, ok)
}

// privateOK reports whether ctx allows dialing private addresses; false when unset.
func privateOK(ctx context.Context) bool {
	ok, _ := ctx.Value(privateOKKey{}).(bool)
	return ok
}

// guardAddr refuses a dial to a blocked address; it runs after DNS resolution, so it sees the real IP.
func guardAddr(ctx context.Context, _, address string, _ syscall.RawConn) error {
	addrPort, err := netip.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("%w: unparsable %q", errBlockedAddr, address)
	}
	if isBlocked(addrPort.Addr(), privateOK(ctx)) {
		return fmt.Errorf("%w: %s", errBlockedAddr, addrPort.Addr())
	}
	return nil
}

// blockedAddr reports whether addr is local, link-local or multicast, or private without privateOK.
func blockedAddr(addr netip.Addr, privateOK bool) bool {
	addr = embeddedIPv4(addr.Unmap())
	if addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsMulticast() || addr.IsUnspecified() || inPrefixes(addr, alwaysBlocked) {
		return true
	}
	if privateOK {
		return false
	}
	return addr.IsPrivate() || inPrefixes(addr, privateRanges)
}

// inPrefixes reports whether addr falls in any of prefixes.
func inPrefixes(addr netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// embeddedIPv4 returns the IPv4 address inside a NAT64 or 6to4 address, else addr.
func embeddedIPv4(addr netip.Addr) netip.Addr {
	b := addr.As16()
	switch {
	case nat64Prefix.Contains(addr):
		return netip.AddrFrom4([4]byte(b[12:16]))
	case sixToFourPrefix.Contains(addr):
		return netip.AddrFrom4([4]byte(b[2:6]))
	}
	return addr
}

// mustParsePrefixes parses CIDRs, panicking on a malformed one.
func mustParsePrefixes(cidrs ...string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(cidrs))
	for _, cidr := range cidrs {
		prefixes = append(prefixes, netip.MustParsePrefix(cidr))
	}
	return prefixes
}
