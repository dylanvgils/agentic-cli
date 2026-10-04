package proxy

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
)

// maxHostLen is the longest DNS name accepted as a credential host.
const maxHostLen = 253

// reservedHeaders are dropped or overridden by the reverse proxy, so injecting them would silently do nothing.
var reservedHeaders = []string{
	"Connection",
	"Content-Length",
	"Host",
	"Keep-Alive",
	"Proxy-Connection",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

// Credential is the headers set on HTTPS requests to Hosts.
type Credential struct {
	Hosts []string     `json:"hosts"` // exact hostnames or IPs; wildcards are rejected
	Rules []InjectRule `json:"rules"`
}

// InjectRule overwrites Header with Value, already formatted by the host.
type InjectRule struct {
	Header string `json:"header"`
	Value  string `json:"value"`
}

// LoadCredentials reads the host-written JSON credential list, rejecting invalid entries and hosts listed twice.
func LoadCredentials(path string) ([]Credential, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read proxy credentials: %w", err)
	}

	var creds []Credential
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse proxy credentials %q: %w", path, err)
	}

	seen := make(map[string]bool)
	for i, cred := range creds {
		if err := validateCredential(cred); err != nil {
			return nil, fmt.Errorf("proxy credential %d: %w", i, err)
		}

		for _, host := range cred.Hosts {
			host = normalizeHost(host)
			if seen[host] {
				return nil, fmt.Errorf("proxy credential %d: host %s is listed more than once", i, host)
			}
			seen[host] = true
		}
	}
	return creds, nil
}

// validateCredential requires exact hosts and rules with valid headers.
func validateCredential(cred Credential) error {
	if len(cred.Hosts) == 0 {
		return fmt.Errorf("no hosts")
	}
	if len(cred.Rules) == 0 {
		return fmt.Errorf("no rules")
	}

	for _, host := range cred.Hosts {
		if !validHost(normalizeHost(host)) {
			return fmt.Errorf("invalid host %q: must be an exact hostname or IP, without wildcards", host)
		}
	}

	for _, rule := range cred.Rules {
		if !validHeaderName(rule.Header) {
			return fmt.Errorf("invalid header name %q", rule.Header)
		}
		if slices.Contains(reservedHeaders, http.CanonicalHeaderKey(rule.Header)) {
			return fmt.Errorf("header %s cannot be injected", rule.Header)
		}
		if !validHeaderValue(rule.Value) {
			return fmt.Errorf("invalid value for header %s", rule.Header)
		}
	}
	return nil
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

// validHeaderName reports whether name is a non-empty RFC 9110 token.
func validHeaderName(name string) bool {
	if name == "" {
		return false
	}

	for _, c := range name {
		if !isAlphaNum(c) && !strings.ContainsRune("!#$%&'*+-.^_`|~", c) {
			return false
		}
	}
	return true
}

// validHeaderValue reports whether value is non-empty and free of control characters other than tab.
func validHeaderValue(value string) bool {
	if value == "" {
		return false
	}

	for _, c := range value {
		if (c < 0x20 && c != '\t') || c == 0x7f {
			return false
		}
	}
	return true
}

// isAlphaNum reports whether c is an ASCII letter or digit.
func isAlphaNum(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
