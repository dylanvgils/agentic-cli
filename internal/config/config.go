package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LogsDirName is the subdirectory under $AGENTIC_HOME where log files are written.
const LogsDirName = "logs"

// CliConfig holds the persisted global agentic config stored in $AGENTIC_HOME/agentic.json.
type CliConfig struct {
	TrustedDirs           []string             `json:"trusted_dirs"`
	Registry              string               `json:"registry,omitempty"`
	DockerContext         string               `json:"docker_context,omitempty"`
	LastUpdateCheck       *time.Time           `json:"last_update_check,omitempty"`
	ProxyLogRetentionDays int                  `json:"proxy_log_retention_days,omitempty"`
	LastToolVersionCheck  map[string]time.Time `json:"last_tool_version_check,omitempty"`
	// ApprovedCredentials maps a .agenticrc.toml path to the CredentialsHash of the entries the user approved
	ApprovedCredentials map[string]string `json:"approved_credentials,omitempty"`
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

// Save writes the config to $AGENTIC_HOME/agentic.json.
func (config *CliConfig) Save(toolHome string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(ConfigFile(toolHome), data, 0o640)
}

// IsTrusted reports whether dir, once symlinks are resolved, is a trusted entry or below one; entries compare as stored, since a link inside one could otherwise widen trust.
func (config *CliConfig) IsTrusted(dir string) bool {
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return false
	}

	for _, trusted := range config.TrustedDirs {
		trusted = filepath.Clean(trusted)

		if realDir == trusted {
			return true
		}

		if strings.HasPrefix(realDir, trusted+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

// Trust appends dir's real path to the trusted directories and saves the config, so retargeting a symlinked dir later needs a new approval.
func (config *CliConfig) Trust(dir, toolHome string) error {
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}

	config.TrustedDirs = append(config.TrustedDirs, realDir)
	return config.Save(toolHome)
}

// CredentialsApproved reports whether the user approved the credential entries with hash in the config file at path.
func (config *CliConfig) CredentialsApproved(path, hash string) bool {
	return config.ApprovedCredentials[approvalKey(path)] == hash
}

// PendingCredentials returns the layers whose credential entries are new or changed since the user last approved them.
func (config *CliConfig) PendingCredentials(layers []RCLayer) []RCLayer {
	var pending []RCLayer
	for _, layer := range layers {
		creds := layer.RC.Run.Proxy.Credentials
		if len(creds) > 0 && !config.CredentialsApproved(layer.Path, CredentialsHash(creds)) {
			pending = append(pending, layer)
		}
	}
	return pending
}

// ApproveCredentials records hash as the approved credential entries for the config file at path and saves the config.
func (config *CliConfig) ApproveCredentials(path, hash, toolHome string) error {
	if config.ApprovedCredentials == nil {
		config.ApprovedCredentials = make(map[string]string)
	}

	config.ApprovedCredentials[approvalKey(path)] = hash
	return config.Save(toolHome)
}

// ConfigFile returns the path of agentic.json, which must never be mounted into a container.
func ConfigFile(toolHome string) string {
	return filepath.Join(toolHome, "agentic.json")
}

// approvalKey returns path with its dir resolved but not the file itself, so a config file linked to another project's gets its own approval.
func approvalKey(path string) string {
	dir := filepath.Dir(path)
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	return filepath.Join(dir, filepath.Base(path))
}
