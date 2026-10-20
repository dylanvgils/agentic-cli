package proxy

import (
	"encoding/json"
	"fmt"
)

// Rule allows Host, exact or a leading-dot/"*." wildcard.
type Rule struct {
	Host string `json:"host"`
}

// LoadRules parses the JSON rule list in value, empty meaning none.
func LoadRules(value string) ([]Rule, error) {
	if value == "" {
		return nil, nil
	}

	var rules []Rule
	if err := json.Unmarshal([]byte(value), &rules); err != nil {
		return nil, fmt.Errorf("parse %s: %w", EnvRules, err)
	}
	return rules, nil
}
