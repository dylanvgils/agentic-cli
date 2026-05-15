package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// AgenticRC holds the parsed contents of a .agenticrc project config file.
type AgenticRC struct {
	ExtraMounts []string
	PidsLimit   string
	CPUs        string
	Memory      string
}

// FindAndLoad walks up from startDir looking for .agenticrc and parses it.
// Returns an empty AgenticRC if no file is found or the file cannot be read.
func FindAndLoad(startDir string) *AgenticRC {
	path, ok := findRC(startDir)
	if !ok {
		return &AgenticRC{}
	}

	rc, err := loadRC(path)
	if err != nil {
		return &AgenticRC{}
	}

	return rc
}

func findRC(startDir string) (string, bool) {
	dir := startDir

	for {
		candidate := filepath.Join(dir, ".agenticrc")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}

		dir = parent
	}
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

func parseRC(f *os.File) (*AgenticRC, error) {
	home, _ := os.UserHomeDir()

	rc := &AgenticRC{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = stripQuotes(value)

		switch key {
		case "EXTRA_MOUNTS":
			if value == "" {
				continue
			}
			parts := strings.Split(value, ",")
			for _, p := range parts {
				p = strings.ReplaceAll(p, "~", home)
				rc.ExtraMounts = append(rc.ExtraMounts, p)
			}
		case "PIDS_LIMIT":
			rc.PidsLimit = value
		case "CPUS":
			rc.CPUs = value
		case "MEMORY":
			rc.Memory = value
		}
	}

	return rc, scanner.Err()
}

func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}

	return s
}
