// Package migrate applies versioned changes to $AGENTIC_HOME's on-disk
// layout. Applied migrations are tracked via a schema version stored at
// TOOL_HOME/.migrations.json, so each entry in registry.go runs at most
// once, in order. The migrations themselves - the plain functions each
// entry's Apply points at - live in internal/migrate/migrations.
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

// state is migrate's own bookkeeping, stored at TOOL_HOME/.migrations.json - separate from the user-facing agentic.json.
type state struct {
	Version int `json:"version"`
}

// Run applies every migration newer than TOOL_HOME's recorded schema version,
// in ascending order, persisting progress after each successful step. It
// returns the migrations actually applied (for logging), or the first error.
func Run(toolHome string) ([]Migration, error) {
	return run(toolHome, registry)
}

// run is Run's implementation, taking the migration list as a parameter so tests can substitute a fixture list without mutating the real registry.
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

// baseline creates a fresh toolHome and records it at the latest schema version without running any migration - a directory that never existed can't be in a legacy shape.
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

// saveState atomically writes s to TOOL_HOME/.migrations.json: write to a temp file in the same directory, then rename over the target.
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
