// Package proxy implements a fail-closed forward proxy that restricts tool containers to an allowed host set and logs every connection attempt.
package proxy

import (
	"maps"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
)

// DefaultPorts are the destination ports tunneled to when an allowlist entry does not pin a specific port.
var DefaultPorts = []string{"80", "443"}

// Allowlist decides whether a (host, port) destination is permitted. A rule matches the host
// exactly, or - with a leading "." or "*." - matches that domain and its subdomains; no substring
// matching is performed, so "anthropic.com" never matches "evil-anthropic.com". A rule with paths limits its host to them.
type Allowlist struct {
	exact    map[string]bool
	suffixes []string            // normalized to ".example.com", matches the domain and subdomains
	paths    map[string][]string // path patterns per exact host, limiting it even where another rule matches it
}

// NewAllowlist builds an Allowlist from rules, skipping blank hosts and normalizing the rest like the hosts they match;
// a host's path rules win over any rule allowing it whole.
func NewAllowlist(rules []Rule) *Allowlist {
	allowList := &Allowlist{exact: make(map[string]bool), paths: make(map[string][]string)}

	for _, rule := range rules {
		entry := normalizeHost(strings.TrimSpace(rule.Host))
		if entry == "" || len(rule.Paths) > 0 {
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

	for _, rule := range rules {
		host := normalizeHost(rule.Host)
		if len(rule.Paths) > 0 {
			allowList.paths[host] = append(allowList.paths[host], rule.Paths...)
		}
	}

	return allowList
}

// Allows reports whether a connection to host on port is permitted: port must be one of DefaultPorts and host must match
// an allowlist entry or have path rules, whose requests AllowsPath then checks.
func (a *Allowlist) Allows(host, port string) bool {
	if !slices.Contains(DefaultPorts, port) {
		return false
	}
	return a.matchesHost(host) || a.pathRestricted(host)
}

// AllowsPath reports whether a request for path, already checked by requestPath, is permitted on host: always for a host
// without path rules, else when one of its patterns matches.
func (a *Allowlist) AllowsPath(host, path string) bool {
	patterns, restricted := a.paths[normalizeHost(host)]
	if !restricted {
		return true
	}
	return slices.ContainsFunc(patterns, func(pattern string) bool { return config.MatchPathPattern(pattern, path) })
}

// matchesHost reports whether host matches an entry, ignoring the port.
func (a *Allowlist) matchesHost(host string) bool {
	host = normalizeHost(host)
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

// exactMatch reports whether host matches an exact entry or path rule, which may then resolve to a private address.
func (a *Allowlist) exactMatch(host string) bool {
	return a.exact[normalizeHost(host)] || a.pathRestricted(host)
}

// pathHosts returns the hosts allowed only on some paths, sorted.
func (a *Allowlist) pathHosts() []string {
	return slices.Sorted(maps.Keys(a.paths))
}

// pathRestricted reports whether host is allowed only on the paths of its rules.
func (a *Allowlist) pathRestricted(host string) bool {
	return len(a.paths[normalizeHost(host)]) > 0
}

// normalizeHost lower-cases host and drops a trailing root dot, so equivalent spellings compare equal.
func normalizeHost(host string) string {
	return strings.ToLower(strings.TrimSuffix(host, "."))
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
