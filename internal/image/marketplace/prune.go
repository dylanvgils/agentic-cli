package marketplace

import (
	"os"
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/config"
)

// PruneKind categorizes the outcome Prune reached for one registry entry or clone dir.
type PruneKind int

const (
	PruneNoRecord PruneKind = iota
	PruneRemoved
	PruneDropped
	PruneKept
)

// PruneResult is one message-worthy outcome from Prune.
type PruneResult struct {
	Kind     PruneKind
	DirName  string
	Name     string   // empty for PruneNoRecord
	Projects []string // populated for PruneKept
}

// LiveProjects re-checks each project's config, dropping any that no longer declare name/dirName.
func LiveProjects(dirName, name string, projects []string) []string {
	var live []string
	for _, dir := range projects {
		rc, err := config.FindAndLoad(dir)
		if err != nil {
			continue
		}
		for _, m := range rc.Marketplaces {
			if m.Name == name && CloneDirName(m.URL) == dirName {
				live = append(live, dir)
				break
			}
		}
	}
	return live
}

// Prune drops reg entries no live project still references, removes clone dirs left empty, and reports the outcome per entry.
func Prune(baseDir string, reg *Registry) (*Registry, []PruneResult, error) {
	dirNames, err := CloneDirs(baseDir)
	if err != nil {
		return nil, nil, err
	}

	updated := &Registry{Marketplaces: map[string][]RegistryEntry{}}
	var report []PruneResult

	for _, dirName := range dirNames {
		entries := reg.Marketplaces[dirName]
		if len(entries) == 0 {
			report = append(report, PruneResult{Kind: PruneNoRecord, DirName: dirName})
			continue
		}

		survivors, results, err := pruneDir(baseDir, dirName, entries)
		if err != nil {
			return nil, nil, err
		}

		report = append(report, results...)
		if len(survivors) > 0 {
			updated.Marketplaces[dirName] = survivors
		}
	}

	return updated, report, nil
}

// pruneDir splits dirName's entries into survivors and dead ones, removing the clone dir once nothing survives.
func pruneDir(baseDir, dirName string, entries []RegistryEntry) ([]RegistryEntry, []PruneResult, error) {
	survivors, dead := splitLiveEntries(dirName, entries)

	if len(survivors) == 0 {
		if err := os.RemoveAll(filepath.Join(baseDir, dirName)); err != nil {
			return nil, nil, err
		}
		return nil, pruneResults(dirName, PruneRemoved, dead), nil
	}

	report := pruneResults(dirName, PruneDropped, dead)
	for _, entry := range survivors {
		report = append(report, PruneResult{Kind: PruneKept, DirName: dirName, Name: entry.Name, Projects: entry.Projects})
	}

	return survivors, report, nil
}

// splitLiveEntries partitions entries by whether any project still references them, trimming survivors' Projects to the live ones.
func splitLiveEntries(dirName string, entries []RegistryEntry) (survivors, dead []RegistryEntry) {
	for _, entry := range entries {
		live := LiveProjects(dirName, entry.Name, entry.Projects)
		if len(live) == 0 {
			dead = append(dead, entry)
			continue
		}
		entry.Projects = live
		survivors = append(survivors, entry)
	}

	return survivors, dead
}

// pruneResults builds one report entry of kind per entry, name-only (no Projects).
func pruneResults(dirName string, kind PruneKind, entries []RegistryEntry) []PruneResult {
	results := make([]PruneResult, len(entries))
	for i, entry := range entries {
		results[i] = PruneResult{Kind: kind, DirName: dirName, Name: entry.Name}
	}

	return results
}
