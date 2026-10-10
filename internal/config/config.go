package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// LogsDirName is the subdirectory under $AGENTIC_HOME where log files are written.
const LogsDirName = "logs"

// CliConfig holds the machine-wide settings users edit in $AGENTIC_HOME/agentic.json; agentic never writes it.
type CliConfig struct {
	Registry              string `json:"registry,omitempty"`
	DockerContext         string `json:"docker_context,omitempty"`
	ProxyLogRetentionDays int    `json:"proxy_log_retention_days,omitempty"`
}

// LoadConfig reads $AGENTIC_HOME/agentic.json, returning an empty CliConfig if the file does not exist.
func LoadConfig(toolHome string) (*CliConfig, error) {
	path := ConfigFile(toolHome)

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &CliConfig{}, nil
	}
	if err != nil {
		return nil, err
	}

	var config CliConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// ConfigFile returns the path of agentic.json, which must never be mounted into a container.
func ConfigFile(toolHome string) string {
	return filepath.Join(toolHome, "agentic.json")
}
