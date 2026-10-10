package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// State holds what agentic records itself in $AGENTIC_HOME/state.json, apart from the settings users edit in agentic.json.
type State struct {
	TrustedDirs          []string             `json:"trusted_dirs,omitempty"`
	LastUpdateCheck      *time.Time           `json:"last_update_check,omitempty"`
	LastToolVersionCheck map[string]time.Time `json:"last_tool_version_check,omitempty"`
	// ApprovedSettings maps a .agenticrc.toml path to the hash of each guarded setting the user approved; a missing key was approved unset
	ApprovedSettings map[string]map[string]string `json:"approved_settings,omitempty"`
}

// LoadState reads $AGENTIC_HOME/state.json, returning an empty State if the file does not exist.
func LoadState(toolHome string) (*State, error) {
	data, err := os.ReadFile(StateFile(toolHome))
	if errors.Is(err, os.ErrNotExist) {
		return &State{}, nil
	}
	if err != nil {
		return nil, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// Save writes the state to $AGENTIC_HOME/state.json.
func (state *State) Save(toolHome string) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(StateFile(toolHome), data, 0o640)
}

// IsTrusted reports whether dir, once symlinks are resolved, is a trusted entry or below one; entries compare as stored, since a link inside one could otherwise widen trust.
func (state *State) IsTrusted(dir string) bool {
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return false
	}

	for _, trusted := range state.TrustedDirs {
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

// Trust appends dir's real path to the trusted directories and saves the state, so retargeting a symlinked dir later needs a new approval.
func (state *State) Trust(dir, toolHome string) error {
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}

	state.TrustedDirs = append(state.TrustedDirs, realDir)
	return state.Save(toolHome)
}

// ChangedSettings returns the guarded settings of layer that differ from what the user last approved for its file; never-approved settings count as approved unset.
func (state *State) ChangedSettings(layer RCLayer) []GuardedSetting {
	approved := state.ApprovedSettings[approvalKey(layer.Path)]

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

// ApproveSettings records layer's guarded settings as approved for its file and saves the state.
func (state *State) ApproveSettings(layer RCLayer, toolHome string) error {
	hashes := make(map[string]string)
	for _, setting := range GuardedSettings(layer.RC) {
		if setting.IsSet() {
			hashes[setting.Key] = setting.hash()
		}
	}

	if state.ApprovedSettings == nil {
		state.ApprovedSettings = make(map[string]map[string]string)
	}
	state.ApprovedSettings[approvalKey(layer.Path)] = hashes
	return state.Save(toolHome)
}

// StateFile returns the path of state.json, which must never be mounted into a container.
func StateFile(toolHome string) string {
	return filepath.Join(toolHome, "state.json")
}

// approvalKey returns path with its dir resolved but not the file itself, so a config file linked to another project's gets its own approval.
func approvalKey(path string) string {
	dir := filepath.Dir(path)
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	return filepath.Join(dir, filepath.Base(path))
}
