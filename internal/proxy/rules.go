package proxy

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/dylanvgils/agentic-cli/internal/config"
)

// Rule allows Host on every path, or with Paths only on those (see config.MatchPathPattern); Host is exact or a
// leading-dot/"*." wildcard, exact when it has Paths.
type Rule struct {
	Host  string   `json:"host"`
	Paths []string `json:"paths,omitempty"`
}

// LoadRules parses the JSON rule list in value, empty meaning none, and validates it.
func LoadRules(value string) ([]Rule, error) {
	if value == "" {
		return nil, nil
	}

	var rules []Rule
	if err := json.Unmarshal([]byte(value), &rules); err != nil {
		return nil, fmt.Errorf("parse %s: %w", EnvRules, err)
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

// ValidateRules requires a rule with paths to have an exact hostname or IP and valid path patterns.
func ValidateRules(rules []Rule) error {
	for _, rule := range rules {
		if len(rule.Paths) == 0 {
			continue
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
