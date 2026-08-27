// Package migrate applies versioned changes to TOOL_HOME's on-disk layout, tracked via TOOL_HOME/.migrations.json.
package migrate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/logging"
)

const stateFileName = ".migrations.json"

// state is migrate's own bookkeeping, stored at TOOL_HOME/.migrations.json.
type state struct {
	Version int `json:"version"`
}

// Run applies every migration newer than TOOL_HOME's recorded schema version, returning what was applied.
func Run(toolHome string) ([]Migration, error) {
	return run(toolHome, changelog)
}

// run is Run's implementation; takes the migration list so tests can use a fixture instead of the real changelog.
func run(toolHome string, pending []Migration) ([]Migration, error) {
	if _, err := os.Stat(toolHome); errors.Is(err, os.ErrNotExist) {
		return nil, baseline(toolHome, pending)
	} else if err != nil {
		return nil, err
	}

	current, err := loadState(toolHome)
	if err != nil {
		return nil, err
	}

	if floor := oldestSupportedVersion(pending); current.Version < floor {
		return nil, fmt.Errorf("migrate: TOOL_HOME is at schema version %d, older than the oldest supported migration (v%d) - cannot upgrade automatically", current.Version, floor+1)
	}

	var applied []Migration
	for _, m := range pending {
		if m.Version <= current.Version {
			continue
		}

		if err := m.Apply(toolHome); err != nil {
			return applied, fmt.Errorf("migrate: applying migration %d (%s): %w", m.Version, m.Description, err)
		}

		current.Version = m.Version
		if err := saveState(toolHome, current); err != nil {
			return applied, err
		}

		logging.Stepf("migrated: %s (v%d)", m.Description, m.Version)
		applied = append(applied, m)
	}

	return applied, nil
}

// baseline creates a fresh toolHome already at the latest schema version - a new directory can't be in a legacy shape.
func baseline(toolHome string, pending []Migration) error {
	if err := os.MkdirAll(toolHome, 0o750); err != nil {
		return err
	}
	return saveState(toolHome, state{Version: latestVersion(pending)})
}

// latestVersion returns the highest Version in pending, or 0 if empty.
func latestVersion(pending []Migration) int {
	latest := 0
	for _, m := range pending {
		if m.Version > latest {
			latest = m.Version
		}
	}
	return latest
}

// oldestSupportedVersion returns one less than pending's oldest Version, or 0 if empty.
func oldestSupportedVersion(pending []Migration) int {
	if len(pending) == 0 {
		return 0
	}
	return pending[0].Version - 1
}

// loadState reads TOOL_HOME/.migrations.json, returning state{Version: 0} if the file does not exist.
func loadState(toolHome string) (state, error) {
	data, err := os.ReadFile(filepath.Join(toolHome, stateFileName))
	if errors.Is(err, os.ErrNotExist) {
		return state{}, nil
	}
	if err != nil {
		return state{}, err
	}

	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return state{}, err
	}

	return s, nil
}

// saveState atomically writes s to TOOL_HOME/.migrations.json via a temp file + rename.
func saveState(toolHome string, s state) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(toolHome, stateFileName+".tmp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmp.Name(), filepath.Join(toolHome, stateFileName))
}
