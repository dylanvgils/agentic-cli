package marketplace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// registryFileName holds the usage registry inside a marketplace baseDir.
const registryFileName = ".usage.json"

// RegistryEntry records what's known about one synced marketplace clone.
type RegistryEntry struct {
	Name     string   `json:"name"`
	URL      string   `json:"url"`
	Projects []string `json:"projects"` // absolute dirs `agentic run` was invoked from
	Stale    bool     `json:"stale"`    // true if the most recent sync fell back to an existing clone
}

// Registry maps CloneDirName(url) to the marketplaces (by local name) sharing that clone.
type Registry struct {
	Marketplaces map[string][]RegistryEntry `json:"marketplaces"`
}

// LoadRegistry reads the usage registry from baseDir. A missing or undecodable file returns an empty Registry, not an error.
func LoadRegistry(baseDir string) (*Registry, error) {
	data, err := os.ReadFile(filepath.Join(baseDir, registryFileName))
	if os.IsNotExist(err) {
		return &Registry{Marketplaces: map[string][]RegistryEntry{}}, nil
	}
	if err != nil {
		return nil, err
	}

	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return &Registry{Marketplaces: map[string][]RegistryEntry{}}, nil
	}
	if reg.Marketplaces == nil {
		reg.Marketplaces = map[string][]RegistryEntry{}
	}

	return &reg, nil
}

// SaveRegistry writes reg to baseDir, creating baseDir if needed.
func SaveRegistry(baseDir string, reg *Registry) error {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(baseDir, registryFileName), data, 0o644)
}

// RecordUsage loads baseDir's registry, records projectDir against each result, and saves it back.
func RecordUsage(baseDir string, results []Result, projectDir string) error {
	reg, err := LoadRegistry(baseDir)
	if err != nil {
		return err
	}

	for _, r := range results {
		key := CloneDirName(r.Entry.URL)
		reg.Record(key, RegistryEntry{Name: r.Entry.Name, URL: r.Entry.URL, Stale: r.Stale}, projectDir)
	}

	return SaveRegistry(baseDir, reg)
}

// Record adds projectDir to key's entry matching entry.Name, creating it if needed (deduped, sorted).
func (reg *Registry) Record(key string, entry RegistryEntry, projectDir string) {
	entries := reg.Marketplaces[key]

	for i := range entries {
		if entries[i].Name != entry.Name {
			continue
		}

		entries[i].URL = entry.URL
		entries[i].Stale = entry.Stale
		if !slices.Contains(entries[i].Projects, projectDir) {
			entries[i].Projects = append(entries[i].Projects, projectDir)
			slices.Sort(entries[i].Projects)
		}

		reg.Marketplaces[key] = entries
		return
	}

	entry.Projects = []string{projectDir}
	entries = append(entries, entry)
	slices.SortFunc(entries, func(a, b RegistryEntry) int { return strings.Compare(a.Name, b.Name) })
	reg.Marketplaces[key] = entries
}
