package marketplace

import (
	"os"
	"sort"
)

// CloneDirs lists baseDir's subdirectories, sorted. Missing baseDir is not an error.
func CloneDirs(baseDir string) ([]string, error) {
	entries, err := os.ReadDir(baseDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	return names, nil
}
