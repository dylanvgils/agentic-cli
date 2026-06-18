// Package proxy implements a fail-closed forward proxy that restricts agentic
// tool containers to a configurable set of allowed hosts and records every
// connection attempt as structured JSON log lines.
package proxy

import (
	"slices"
	"strings"
)

// DefaultPorts are the destination ports a forward proxy will tunnel to when an
// allowlist entry does not pin a specific port.
var DefaultPorts = []string{"80", "443"}

// Allowlist decides whether a (host, port) destination is permitted.
//
// An entry matches the host exactly (e.g. "api.anthropic.com"), or - when it
// starts with a leading dot or "*." - matches that domain and any subdomain
// (e.g. ".anthropic.com" matches "anthropic.com" and "api.anthropic.com"). No
// substring matching is performed, so "anthropic.com" never matches
// "evil-anthropic.com".
type Allowlist struct {
	exact    map[string]bool
	suffixes []string // normalized to ".example.com", matches the domain and subdomains
}

// NewAllowlist builds an Allowlist from raw entries. Empty and blank entries are
// ignored; entries are lower-cased so matching is case-insensitive.
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

// Allows reports whether a connection to host on port is permitted. The port
// must be one of DefaultPorts and the host must match an allowlist entry.
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

// wildcardSuffix returns the normalized ".example.com" suffix for a wildcard
// entry (".example.com" or "*.example.com"), or ok=false for an exact entry.
func wildcardSuffix(entry string) (string, bool) {
	if after, ok := strings.CutPrefix(entry, "*."); ok {
		return "." + after, true
	}
	if strings.HasPrefix(entry, ".") {
		return entry, true
	}
	return "", false
}
