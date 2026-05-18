// Package config provides project configuration loaded from .agenticrc files.
package config

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// AgenticRC holds the parsed contents of a .agenticrc project config file.
type AgenticRC struct {
	Root        bool
	ExtraMounts []string
	Secrets     []string
	PidsLimit   string
	CPUs        string
	Memory      string
}

// RCLayer pairs a parsed .agenticrc with the path it was loaded from.
type RCLayer struct {
	Path string
	RC   *AgenticRC
}

// FindAndLoad walks up from startDir collecting all .agenticrc files and merges
// them. Stops when a file with root=true is encountered. For scalar keys the
// innermost (child) value wins; list keys accumulate outermost-first.
// Returns an empty AgenticRC if no file is found.
func FindAndLoad(startDir string) *AgenticRC {
	paths := collectPaths(startDir)
	configs := loadConfigs(paths)
	return mergeConfigs(configs)
}

// FindLayers returns the .agenticrc layers that FindAndLoad would merge,
// ordered outermost-to-innermost, each paired with its source path.
func FindLayers(startDir string) []RCLayer {
	paths := collectPaths(startDir)
	var layers []RCLayer

	for _, path := range paths {
		rc, err := loadRC(path)
		if err != nil {
			continue
		}

		layers = append(layers, RCLayer{Path: path, RC: rc})
		if rc.Root {
			break
		}
	}

	for i, j := 0, len(layers)-1; i < j; i, j = i+1, j-1 {
		layers[i], layers[j] = layers[j], layers[i]
	}

	return layers
}

func collectPaths(startDir string) []string {
	var paths []string
	dir := startDir

	for {
		candidate := filepath.Join(dir, ".agenticrc")
		if _, err := os.Stat(candidate); err == nil {
			paths = append(paths, candidate)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return paths
}

func loadConfigs(paths []string) []*AgenticRC {
	var configs []*AgenticRC

	for _, path := range paths {
		rc, err := loadRC(path)
		if err != nil {
			continue
		}
		configs = append(configs, rc)

		if rc.Root {
			break
		}
	}

	return configs
}

func mergeConfigs(configs []*AgenticRC) *AgenticRC {
	result := &AgenticRC{}

	for _, rc := range configs {
		if result.PidsLimit == "" {
			result.PidsLimit = rc.PidsLimit
		}

		if result.CPUs == "" {
			result.CPUs = rc.CPUs
		}

		if result.Memory == "" {
			result.Memory = rc.Memory
		}
	}

	for i := len(configs) - 1; i >= 0; i-- {
		result.ExtraMounts = append(result.ExtraMounts, configs[i].ExtraMounts...)
		result.Secrets = append(result.Secrets, configs[i].Secrets...)
	}

	return result
}

func loadRC(path string) (*AgenticRC, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	rc, err := parseRC(f)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}

	return rc, err
}

func parseRC(r io.Reader) (*AgenticRC, error) {
	rc := &AgenticRC{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = stripQuotes(strings.TrimSpace(value))

		switch key {
		case "root":
			rc.Root = value == "true"

		case "extra_mounts":
			rc.ExtraMounts = append(rc.ExtraMounts, splitValues(value)...)
		case "secrets":
			rc.Secrets = append(rc.Secrets, splitValues(value)...)

		case "pids_limit":
			rc.PidsLimit = value
		case "cpus":
			rc.CPUs = value
		case "memory":
			rc.Memory = value
		}
	}

	return rc, scanner.Err()
}

// splitValues splits a comma-separated value string and skips empty parts.
// Supports both comma-separated and repeatable-key styles. Variable expansion
// is handled later by mount.ExpandVars at container launch time.
func splitValues(value string) []string {
	var result []string

	for pair := range strings.SplitSeq(value, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		result = append(result, pair)
	}

	return result
}

func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}

	return s
}
