// Package agentichome owns the files agentic keeps in $AGENTIC_HOME: agentic.json, state.json and the custom-installs store.
package agentichome

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Env names the variable that overrides the agentic data dir.
const Env = "AGENTIC_HOME"

// defaultLogRetentionDays is how long log dirs are kept when agentic.json sets no retention.
const defaultLogRetentionDays = 3

// Config holds the machine-wide settings users edit in $AGENTIC_HOME/agentic.json; agentic never writes it.
type Config struct {
	Registry         string `json:"registry,omitempty"`
	DockerContext    string `json:"docker_context,omitempty"`
	LogRetentionDays int    `json:"log_retention_days,omitempty"`
}

// LoadConfig reads $AGENTIC_HOME/agentic.json, returning an empty Config if the file does not exist.
func LoadConfig(toolHome string) (*Config, error) {
	path := ConfigFile(toolHome)

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// LogRetention returns how long log dirs are kept: log_retention_days, else the default.
func (c *Config) LogRetention() time.Duration {
	days := c.LogRetentionDays
	if days <= 0 {
		days = defaultLogRetentionDays
	}
	return time.Duration(days) * 24 * time.Hour
}

// ConfigFile returns the path of agentic.json, which must never be mounted into a container.
func ConfigFile(toolHome string) string {
	return filepath.Join(toolHome, "agentic.json")
}
