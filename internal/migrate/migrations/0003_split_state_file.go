package migrations

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// stateKeys is a historical snapshot of the keys moved, not config.State's fields - it must not track future state changes.
var stateKeys = []string{"trusted_dirs", "last_update_check", "last_tool_version_check", "approved_settings"}

// droppedKeys are obsolete keys nothing reads anymore; credential approvals are now asked for again as a guarded setting.
var droppedKeys = []string{"approved_credentials"}

// SplitStateFile moves the keys agentic writes itself from agentic.json to state.json and drops obsolete ones, keeping agentic.json's other keys. Safe to retry after a partial failure.
func SplitStateFile(toolHome string) error {
	configPath := filepath.Join(toolHome, "agentic.json")
	statePath := filepath.Join(toolHome, "state.json")

	settings, err := readJSONObject(configPath)
	if err != nil {
		return err
	}

	dropped := false
	for _, key := range droppedKeys {
		if _, ok := settings[key]; ok {
			delete(settings, key)
			dropped = true
		}
	}

	moved := make(map[string]json.RawMessage)
	for _, key := range stateKeys {
		if value, ok := settings[key]; ok {
			moved[key] = value
			delete(settings, key)
		}
	}
	if len(moved) == 0 && !dropped {
		return nil
	}

	// Written first, so a failure below leaves stale keys in agentic.json rather than lost state
	if len(moved) > 0 {
		if err := mergeState(statePath, moved); err != nil {
			return err
		}
	}
	return writeJSONObject(configPath, settings)
}

// mergeState adds moved to the state.json at path, where values a retry finds from before a failure win.
func mergeState(path string, moved map[string]json.RawMessage) error {
	state, err := readJSONObject(path)
	if err != nil {
		return err
	}
	for key, value := range moved {
		if _, ok := state[key]; !ok {
			state[key] = value
		}
	}
	return writeJSONObject(path, state)
}

// readJSONObject reads the JSON object at path, or an empty one if the file does not exist.
func readJSONObject(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, err
	}

	object := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	return object, nil
}

// writeJSONObject writes object to path as indented JSON.
func writeJSONObject(path string, object map[string]json.RawMessage) error {
	data, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o640)
}
