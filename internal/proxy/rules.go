package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
)

// Rule allows Host on every path, or with Paths only on those (see config.MatchPathPattern); Host is exact or a
// leading-dot/"*." wildcard, exact when it has Paths.
type Rule struct {
	Host  string   `json:"host"`
	Paths []string `json:"paths,omitempty"`
}

// LoadRules parses and validates the JSON rule list in value, empty meaning none, refusing unknown fields.
func LoadRules(value string) ([]Rule, error) {
	if value == "" {
		return nil, nil
	}

	var rules []Rule
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rules); err != nil {
		return nil, fmt.Errorf("parse %s: %w", EnvRules, err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse %s: unexpected data after the rule list", EnvRules)
	}
	if err := ValidateRules(rules); err != nil {
		return nil, fmt.Errorf("%s: %w", EnvRules, err)
	}
	return rules, nil
}

// PathHosts returns the host of every rule with paths, normalized and without duplicates.
func PathHosts(rules []Rule) []string {
	var hosts []string
	for _, rule := range rules {
		host := normalizeHost(rule.Host)
		if len(rule.Paths) > 0 && !slices.Contains(hosts, host) {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

// ValidateRules requires a rule with paths to have an exact hostname or IP and a non-empty list of valid patterns.
func ValidateRules(rules []Rule) error {
	for _, rule := range rules {
		if rule.Paths == nil {
			continue
		}
		if len(rule.Paths) == 0 {
			return fmt.Errorf("host %s: paths must not be empty", rule.Host)
		}

		if !validHost(normalizeHost(rule.Host)) {
			return fmt.Errorf("invalid host %q: a host with paths must be an exact hostname or IP, without wildcards", rule.Host)
		}
		for _, pattern := range rule.Paths {
			if err := config.ValidatePathPattern(pattern); err != nil {
				return fmt.Errorf("host %s: %w", rule.Host, err)
			}
		}
	}
	return nil
}
