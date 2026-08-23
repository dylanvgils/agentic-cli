// Package proxy implements a fail-closed forward proxy that restricts tool containers to an allowed host set and logs every connection attempt.
package proxy

import (
	"slices"
	"strings"
)

// DefaultPorts are the destination ports tunneled to when an allowlist entry does not pin a specific port.
var DefaultPorts = []string{"80", "443"}

// Allowlist decides whether a (host, port) destination is permitted. An entry matches the host
// exactly, or - with a leading "." or "*." - matches that domain and its subdomains; no substring
// matching is performed, so "anthropic.com" never matches "evil-anthropic.com".
type Allowlist struct {
	exact    map[string]bool
	suffixes []string // normalized to ".example.com", matches the domain and subdomains
}

// NewAllowlist builds an Allowlist from raw entries, skipping blank ones and lower-casing the rest for case-insensitive matching.
func NewAllowlist(entries []string) *Allowlist {
	allowList := &Allowlist{exact: make(map[string]bool)}

	for _, raw := range entries {
		entry := strings.ToLower(strings.TrimSpace(raw))
		if entry == "" {
			continue
		}

		if suffix, ok := wildcardSuffix(entry); ok {
			if !slices.Contains(allowList.suffixes, suffix) {
				allowList.suffixes = append(allowList.suffixes, suffix)
			}
			continue
		}

		allowList.exact[entry] = true
	}

	return allowList
}

// Allows reports whether a connection to host on port is permitted: port must be one of DefaultPorts and host must match an allowlist entry.
func (a *Allowlist) Allows(host, port string) bool {
	if !slices.Contains(DefaultPorts, port) {
		return false
	}

	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if a.exact[host] {
		return true
	}

	for _, suffix := range a.suffixes {
		// suffix is ".example.com"; match the bare domain and any subdomain.
		if host == suffix[1:] || strings.HasSuffix(host, suffix) {
			return true
		}
	}

	return false
}

// wildcardSuffix returns the normalized ".example.com" suffix for a wildcard entry, or ok=false for an exact entry.
func wildcardSuffix(entry string) (string, bool) {
	if after, ok := strings.CutPrefix(entry, "*."); ok {
		return "." + after, true
	}
	if strings.HasPrefix(entry, ".") {
		return entry, true
	}
	return "", false
}
