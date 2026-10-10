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
	// ApprovedSettings maps a .agenticrc.toml path to the hash of each guarded setting the user approved; a missing key was approved unset
	ApprovedSettings map[string]map[string]string `json:"approved_settings,omitempty"`
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

// ChangedSettings returns the guarded settings of layer that differ from what the user last approved for its file; never-approved settings count as approved unset.
func (config *CliConfig) ChangedSettings(layer RCLayer) []GuardedSetting {
	approved := config.ApprovedSettings[approvalKey(layer.Path)]

	var changed []GuardedSetting
	for _, setting := range GuardedSettings(layer.RC) {
		want, ok := approved[setting.Key]
		if !ok {
			want = GuardedSetting{}.hash()
		}
		if setting.hash() != want {
			changed = append(changed, setting)
		}
	}
	return changed
}

// ApproveSettings records layer's guarded settings as approved for its file and saves the config.
func (config *CliConfig) ApproveSettings(layer RCLayer, toolHome string) error {
	hashes := make(map[string]string)
	for _, setting := range GuardedSettings(layer.RC) {
		if setting.IsSet() {
			hashes[setting.Key] = setting.hash()
		}
	}

	if config.ApprovedSettings == nil {
		config.ApprovedSettings = make(map[string]map[string]string)
	}
	config.ApprovedSettings[approvalKey(layer.Path)] = hashes
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
