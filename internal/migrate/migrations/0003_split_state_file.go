package migrations

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// stateKeys is a historical snapshot of the keys moved, not config.State's fields - it must not track future state changes.
var stateKeys = []string{"trusted_dirs", "last_update_check", "last_tool_version_check", "approved_settings"}

// SplitStateFile moves the keys agentic writes itself from agentic.json to state.json, keeping agentic.json's other keys. Safe to retry after a partial failure.
func SplitStateFile(toolHome string) error {
	configPath := filepath.Join(toolHome, "agentic.json")
	statePath := filepath.Join(toolHome, "state.json")

	settings, err := readJSONObject(configPath)
	if err != nil {
		return err
	}

	moved := make(map[string]json.RawMessage)
	for _, key := range stateKeys {
		if value, ok := settings[key]; ok {
			moved[key] = value
			delete(settings, key)
		}
	}
	if len(moved) == 0 {
		return nil
	}

	// A retry finds the state.json written before the failure; its values win
	state, err := readJSONObject(statePath)
	if err != nil {
		return err
	}
	for key, value := range moved {
		if _, ok := state[key]; !ok {
			state[key] = value
		}
	}

	// Written first, so a failure below leaves stale keys in agentic.json rather than lost state
	if err := writeJSONObject(statePath, state); err != nil {
		return err
	}
	return writeJSONObject(configPath, settings)
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
