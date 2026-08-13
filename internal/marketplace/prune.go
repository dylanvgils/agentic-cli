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

// PruneAction is one message-worthy outcome from Prune.
type PruneAction struct {
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

// Prune walks baseDir's clone dirs, drops reg entries no live project still references,
// removes clone dirs left with no surviving entries, and reports what happened to each.
func Prune(baseDir string, reg *Registry) (*Registry, []PruneAction, error) {
	dirNames, err := CloneDirs(baseDir)
	if err != nil {
		return nil, nil, err
	}

	updated := &Registry{Marketplaces: map[string][]RegistryEntry{}}
	var report []PruneAction

	for _, dirName := range dirNames {
		entries := reg.Marketplaces[dirName]
		if len(entries) == 0 {
			report = append(report, PruneAction{Kind: PruneNoRecord, DirName: dirName})
			continue
		}

		var survivors, dead []RegistryEntry
		for _, entry := range entries {
			live := LiveProjects(dirName, entry.Name, entry.Projects)
			if len(live) == 0 {
				dead = append(dead, entry)
				continue
			}
			entry.Projects = live
			survivors = append(survivors, entry)
		}

		if len(survivors) == 0 {
			if err := os.RemoveAll(filepath.Join(baseDir, dirName)); err != nil {
				return nil, nil, err
			}
			for _, entry := range dead {
				report = append(report, PruneAction{Kind: PruneRemoved, DirName: dirName, Name: entry.Name})
			}
			continue
		}

		for _, entry := range dead {
			report = append(report, PruneAction{Kind: PruneDropped, DirName: dirName, Name: entry.Name})
		}
		for _, entry := range survivors {
			report = append(report, PruneAction{Kind: PruneKept, DirName: dirName, Name: entry.Name, Projects: entry.Projects})
		}
		updated.Marketplaces[dirName] = survivors
	}

	return updated, report, nil
}
