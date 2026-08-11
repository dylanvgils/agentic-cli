package marketplace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
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

// Registry maps CloneDirName(name, url) to which projects reference it, so
// pruning isn't limited to the current directory's config.
type Registry struct {
	Marketplaces map[string]RegistryEntry `json:"marketplaces"`
}

// LoadRegistry reads the usage registry from baseDir. A missing file returns
// an empty Registry, not an error.
func LoadRegistry(baseDir string) (*Registry, error) {
	data, err := os.ReadFile(filepath.Join(baseDir, registryFileName))
	if os.IsNotExist(err) {
		return &Registry{Marketplaces: map[string]RegistryEntry{}}, nil
	}
	if err != nil {
		return nil, err
	}

	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, err
	}
	if reg.Marketplaces == nil {
		reg.Marketplaces = map[string]RegistryEntry{}
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

// Record adds projectDir to key's entry (deduped, sorted) and refreshes Name,
// URL, and Stale from entry.
func (reg *Registry) Record(key string, entry RegistryEntry, projectDir string) {
	existing := reg.Marketplaces[key]
	existing.Name = entry.Name
	existing.URL = entry.URL
	existing.Stale = entry.Stale

	if !slices.Contains(existing.Projects, projectDir) {
		existing.Projects = append(existing.Projects, projectDir)
		slices.Sort(existing.Projects)
	}

	reg.Marketplaces[key] = existing
}
