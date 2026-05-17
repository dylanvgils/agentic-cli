package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// CliConfig holds the persisted global agentic config stored in $AGENTIC_HOME/agentic.json.
type CliConfig struct {
	TrustedDirs []string `json:"trusted_dirs"`
}

// LoadConfig reads $AGENTIC_HOME/agentic.json. Returns an empty CliConfig if the
// file does not exist.
func LoadConfig(toolHome string) (*CliConfig, error) {
	path := filepath.Join(toolHome, "agentic.json")

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

// Save writes the config to $AGENTIC_HOME/agentic.json.
func (config *CliConfig) Save(toolHome string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(toolHome, "agentic.json"), data, 0o640)
}

// IsTrusted reports whether dir is trusted. An exact match or a match where
// a trusted entry is a parent of dir (separated by filepath.Separator) returns
// true.
func (config *CliConfig) IsTrusted(dir string) bool {
	for _, trusted := range config.TrustedDirs {
		if dir == trusted {
			return true
		}

		if strings.HasPrefix(dir, trusted+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

// Trust appends dir to the trusted directories and saves the config.
func (config *CliConfig) Trust(dir, toolHome string) error {
	config.TrustedDirs = append(config.TrustedDirs, dir)
	return config.Save(toolHome)
}
