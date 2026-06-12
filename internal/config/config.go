package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CliConfig holds the persisted global agentic config stored in $AGENTIC_HOME/agentic.json.
type CliConfig struct {
	TrustedDirs     []string   `json:"trusted_dirs"`
	Registry        string     `json:"registry,omitempty"`
	LastUpdateCheck *time.Time `json:"last_update_check,omitempty"`
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
// true. Symlinks are resolved on both sides so that e.g. /var and /private/var
// on macOS compare equal.
func (config *CliConfig) IsTrusted(dir string) bool {
	realDir := evalSymlinks(dir)

	for _, trusted := range config.TrustedDirs {
		realTrusted := evalSymlinks(trusted)

		if realDir == realTrusted {
			return true
		}

		if strings.HasPrefix(realDir, realTrusted+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

// evalSymlinks resolves symlinks in path, falling back to path on error.
func evalSymlinks(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}

// Trust appends dir to the trusted directories and saves the config.
func (config *CliConfig) Trust(dir, toolHome string) error {
	config.TrustedDirs = append(config.TrustedDirs, dir)
	return config.Save(toolHome)
}
