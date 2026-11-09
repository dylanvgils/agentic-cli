package dind

import (
	"fmt"
	"net/netip"
)

// addressPoolSize is each inner network's prefix length.
const addressPoolSize = 24

// addressPoolCandidates are tried in order; they avoid Docker's and Podman's default ranges.
var addressPoolCandidates = []netip.Prefix{
	netip.MustParsePrefix("10.200.0.0/16"),
	netip.MustParsePrefix("10.201.0.0/16"),
	netip.MustParsePrefix("10.202.0.0/16"),
}

// AddressPool returns dockerd's --default-address-pool value, clear of the run network's subnets: an overlapping inner
// network would hijack the sidecar's route back to the tool.
func AddressPool(runSubnets []string) (string, error) {
	taken := make([]netip.Prefix, 0, len(runSubnets))
	for _, subnet := range runSubnets {
		prefix, err := netip.ParsePrefix(subnet)
		if err != nil {
			return "", fmt.Errorf("parse run network subnet: %w", err)
		}
		taken = append(taken, prefix)
	}

	for _, candidate := range addressPoolCandidates {
		if !overlapsAny(candidate, taken) {
			return fmt.Sprintf("base=%s,size=%d", candidate, addressPoolSize), nil
		}
	}
	return "", fmt.Errorf("no inner address pool is free of the run network subnets %v", runSubnets)
}

// overlapsAny reports whether prefix overlaps any of others.
func overlapsAny(prefix netip.Prefix, others []netip.Prefix) bool {
	for _, other := range others {
		if prefix.Overlaps(other) {
			return true
		}
	}
	return false
}
