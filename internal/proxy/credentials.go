package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Credential is the headers set on HTTPS requests to Hosts.
type Credential struct {
	Hosts []string     `json:"hosts"` // matched like Allowlist entries
	Rules []InjectRule `json:"rules"`
}

// InjectRule overwrites Header with Value, already formatted by the host.
type InjectRule struct {
	Header string `json:"header"`
	Value  string `json:"value"`
}

// LoadCredentials reads the host-written JSON credential list, rejecting incomplete entries.
func LoadCredentials(path string) ([]Credential, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read proxy credentials: %w", err)
	}

	var creds []Credential
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse proxy credentials %q: %w", path, err)
	}

	for i, cred := range creds {
		if err := validateCredential(cred); err != nil {
			return nil, fmt.Errorf("proxy credential %d: %w", i, err)
		}
	}
	return creds, nil
}

// validateCredential requires hosts and rules with valid headers.
func validateCredential(cred Credential) error {
	if len(cred.Hosts) == 0 {
		return fmt.Errorf("no hosts")
	}
	if len(cred.Rules) == 0 {
		return fmt.Errorf("no rules")
	}

	for _, rule := range cred.Rules {
		if rule.Header == "" || strings.ContainsAny(rule.Header, " \t\r\n:") {
			return fmt.Errorf("invalid header name %q", rule.Header)
		}
		if rule.Value == "" || strings.ContainsAny(rule.Value, "\r\n") {
			return fmt.Errorf("invalid value for header %s", rule.Header)
		}
	}
	return nil
}
