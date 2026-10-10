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

// reasonBlockedAddr is the log reason for an allowed host refused by the address guard.
const reasonBlockedAddr = "blocked-address"

var (
	// errBlockedAddr marks a dial refused because the destination resolved to a local or private address.
	errBlockedAddr = errors.New("destination resolves to a blocked address")

	// alwaysBlocked holds ranges the stdlib checks miss; 0.0.0.0/8 reaches the proxy itself on Linux.
	alwaysBlocked = []netip.Prefix{netip.MustParsePrefix("0.0.0.0/8")}

	// privateRanges holds private ranges IsPrivate misses (CGNAT).
	privateRanges = []netip.Prefix{netip.MustParsePrefix("100.64.0.0/10")}

	// upstreamDialer checks each resolved address before connecting, so DNS can't point an allowed name inward.
	upstreamDialer = &net.Dialer{Timeout: dialTimeout, ControlContext: guardAddr}

	// upstreamTransport forwards plain HTTP requests through upstreamDialer.
	upstreamTransport = &http.Transport{
		DialContext:     upstreamDialer.DialContext,
		MaxIdleConns:    100,
		IdleConnTimeout: 90 * time.Second,
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
	addr = addr.Unmap()
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
