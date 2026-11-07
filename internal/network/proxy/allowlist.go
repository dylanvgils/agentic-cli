// Package proxy is a fail-closed forward proxy that limits a run to allowed hosts and logs every attempt.
package proxy

import (
	"net"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
)

// maxHostLen is the longest DNS name accepted as an exact host.
const maxHostLen = 253

// DefaultPorts are the only destination ports the proxy allows.
var DefaultPorts = []string{"80", "443"}

// Allowlist matches hosts exactly or, with a leading "." or "*.", a domain and its subdomains; never by substring.
type Allowlist struct {
	exact    map[string]bool
	suffixes []string            // normalized to ".example.com", matches the domain and subdomains
	paths    map[string][]string // path patterns per exact host, limiting it even where another rule matches it
}

// NewAllowlist builds an Allowlist from rules; a host's path rules win over rules allowing it whole.
func NewAllowlist(rules []Rule) *Allowlist {
	allowList := &Allowlist{exact: make(map[string]bool), paths: make(map[string][]string)}

	for _, rule := range rules {
		entry := normalizeHost(strings.TrimSpace(rule.Host))
		if entry == "" {
			continue
		}

		if len(rule.Paths) > 0 {
			allowList.paths[entry] = append(allowList.paths[entry], rule.Paths...)
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

// Allows reports whether host on port is allowed: a default port and a matching or path rule host.
func (a *Allowlist) Allows(host, port string) bool {
	if !slices.Contains(DefaultPorts, port) {
		return false
	}
	return a.matchesHost(host) || a.pathRestricted(host)
}

// AllowsPath reports whether path is allowed on host; always true for a host without path rules.
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

// pathRestricted reports whether host is allowed only on the paths of its rules.
func (a *Allowlist) pathRestricted(host string) bool {
	return len(a.paths[normalizeHost(host)]) > 0
}

// normalizeHost lower-cases host and drops a trailing root dot, so equivalent spellings compare equal.
func normalizeHost(host string) string {
	return strings.ToLower(strings.TrimSuffix(host, "."))
}

// validHost reports whether host is an IP or a DNS name of letters, digits and inner hyphens.
func validHost(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	if host == "" || len(host) > maxHostLen {
		return false
	}

	for label := range strings.SplitSeq(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !isAlphaNum(c) && c != '-' {
				return false
			}
		}
	}
	return true
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
