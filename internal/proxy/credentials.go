package proxy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
)

// maxHostLen is the longest DNS name accepted as a credential host.
const maxHostLen = 253

// minPlaceholderLen keeps placeholders long enough that no real header value contains one by chance.
const minPlaceholderLen = 16

// redacted replaces a secret wherever a credential is printed or logged.
const redacted = "[redacted]"

// reservedHeaders are dropped or overridden by the reverse proxy, so a secret in them would silently go nowhere.
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

// Credential is a secret for HTTPS requests to Hosts: the proxy swaps Placeholder for Secret in the tool's own Headers.
type Credential struct {
	Hosts       []string `json:"hosts"` // exact hostnames or IPs; wildcards are rejected
	Headers     []string `json:"headers"`
	Placeholder string   `json:"placeholder"`
	Secret      string   `json:"secret"`
}

// String hides Secret, so printing a credential never leaks it.
func (c Credential) String() string {
	return fmt.Sprintf("{Hosts:%v Headers:%v Placeholder:%s Secret:%s}", c.Hosts, c.Headers, c.Placeholder, redactedIfSet(c.Secret))
}

// GoString hides Secret from %#v.
func (c Credential) GoString() string {
	return fmt.Sprintf("proxy.Credential{Hosts:%#v, Headers:%#v, Placeholder:%q, Secret:%q}", c.Hosts, c.Headers, c.Placeholder, redactedIfSet(c.Secret))
}

// LogValue hides Secret from slog.
func (c Credential) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Any("hosts", c.Hosts),
		slog.Any("headers", c.Headers),
		slog.String("placeholder", c.Placeholder),
		slog.String("secret", redactedIfSet(c.Secret)),
	)
}

// CredentialHosts returns every host in creds, normalized and without duplicates.
func CredentialHosts(creds []Credential) []string {
	var hosts []string
	for _, cred := range creds {
		for _, host := range cred.Hosts {
			host = normalizeHost(host)
			if !slices.Contains(hosts, host) {
				hosts = append(hosts, host)
			}
		}
	}
	return hosts
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

	if err := ValidateCredentials(creds); err != nil {
		return nil, err
	}
	return creds, nil
}

// ValidateCredentials rejects invalid entries, hosts listed twice and placeholders that overlap; errors never contain
// secrets.
func ValidateCredentials(creds []Credential) error {
	for i, cred := range creds {
		if err := ValidateCredential(cred); err != nil {
			return fmt.Errorf("proxy credential %d: %w", i, err)
		}
	}

	if err := validateUniqueHosts(creds); err != nil {
		return err
	}
	return validatePlaceholders(creds)
}

// ValidateCredential requires exact hosts, valid headers, a placeholder no real value contains by chance and a secret a
// header can carry; errors never contain the secret.
func ValidateCredential(cred Credential) error {
	if err := validateHosts(cred.Hosts); err != nil {
		return err
	}
	if err := validateHeaders(cred.Headers); err != nil {
		return err
	}

	if !validPlaceholder(cred.Placeholder) {
		return fmt.Errorf("placeholder must be at least %d letters, digits, - or _", minPlaceholderLen)
	}
	if !validHeaderValue(cred.Secret) {
		return fmt.Errorf("invalid secret")
	}
	return nil
}

// validateUniqueHosts rejects a host listed in more than one credential.
func validateUniqueHosts(creds []Credential) error {
	seen := make(map[string]bool)
	for _, cred := range creds {
		for _, host := range cred.Hosts {
			host = normalizeHost(host)
			if seen[host] {
				return fmt.Errorf("host %s is listed in more than one credential", host)
			}
			seen[host] = true
		}
	}
	return nil
}

// validatePlaceholders rejects a placeholder inside another, which would swap in the wrong secret.
func validatePlaceholders(creds []Credential) error {
	for i, a := range creds {
		for _, b := range creds[i+1:] {
			if strings.Contains(a.Placeholder, b.Placeholder) || strings.Contains(b.Placeholder, a.Placeholder) {
				return fmt.Errorf("placeholders %s and %s overlap", a.Placeholder, b.Placeholder)
			}
		}
	}
	return nil
}

// validateHosts requires at least one host, each an exact hostname or IP.
func validateHosts(hosts []string) error {
	if len(hosts) == 0 {
		return fmt.Errorf("no hosts")
	}

	for _, host := range hosts {
		if !validHost(normalizeHost(host)) {
			return fmt.Errorf("invalid host %q: must be an exact hostname or IP, without wildcards", host)
		}
	}
	return nil
}

// validateHeaders requires at least one header, each one that can be injected.
func validateHeaders(headers []string) error {
	if len(headers) == 0 {
		return fmt.Errorf("no headers")
	}

	for _, header := range headers {
		if err := validateHeader(header); err != nil {
			return err
		}
	}
	return nil
}

// validateHeader rejects a malformed header name or one the reverse proxy drops or overrides.
func validateHeader(name string) error {
	if !validHeaderName(name) {
		return fmt.Errorf("invalid header name %q", name)
	}
	if slices.Contains(reservedHeaders, http.CanonicalHeaderKey(name)) {
		return fmt.Errorf("header %s cannot be injected", name)
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

// validPlaceholder reports whether placeholder is long enough and only letters, digits, - or _, so it survives any header
// value and Basic encoding unchanged.
func validPlaceholder(placeholder string) bool {
	if len(placeholder) < minPlaceholderLen {
		return false
	}
	for _, c := range placeholder {
		if !isAlphaNum(c) && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

// redactedIfSet hides a non-empty secret and keeps an empty one visible as empty.
func redactedIfSet(secret string) string {
	if secret == "" {
		return ""
	}
	return redacted
}

// isAlphaNum reports whether c is an ASCII letter or digit.
func isAlphaNum(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
