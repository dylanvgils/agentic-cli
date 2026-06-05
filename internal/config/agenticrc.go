// Package config provides project configuration loaded from .agenticrc.toml files.
package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// RCBuild holds build-time settings from a .agenticrc.toml file.
type RCBuild struct {
	AptPackages []string `toml:"apt_packages"`
}

// RCRun holds runtime settings from a .agenticrc.toml file.
type RCRun struct {
	ExtraMounts []string `toml:"extra_mounts"`
	Secrets     []string `toml:"secrets"`
	PidsLimit   string   `toml:"pids_limit"`
	CPUs        string   `toml:"cpus"`
	Memory      string   `toml:"memory"`
}

// AgenticRC holds the parsed contents of a .agenticrc.toml project config file.
type AgenticRC struct {
	Root      bool     `toml:"root"`
	Namespace string   `toml:"namespace"`
	Build     RCBuild  `toml:"build"`
	Run       RCRun    `toml:"run"`
}

// RCLayer pairs a parsed .agenticrc.toml with the path it was loaded from.
type RCLayer struct {
	Path string
	RC   *AgenticRC
}

const rcFilename = ".agenticrc.toml"
const legacyRCFilename = ".agenticrc"

// rcWarningWriter is the destination for legacy-file warnings; overridable in tests.
var rcWarningWriter io.Writer = os.Stderr

// FindAndLoadFromCwd loads config starting from the current working directory.
func FindAndLoadFromCwd() *AgenticRC {
	cwd, _ := os.Getwd()
	return FindAndLoad(cwd)
}

// AptPackages returns the merged apt packages from .agenticrc.toml files and the
// AGENTIC_APT_PACKAGES env var, outermost RC first, env var last.
func AptPackages(startDir string) []string {
	rcPkgs := FindAndLoad(startDir).Build.AptPackages
	envPkgs := splitEnvValues(os.Getenv(EnvAptPackages))
	return append(rcPkgs, envPkgs...)
}

// FindAndLoad walks up from startDir collecting all .agenticrc.toml files and
// merges them. Stops when a file with root=true is encountered. For scalar keys
// the innermost (child) value wins; list keys accumulate outermost-first.
// Returns an empty AgenticRC if no file is found.
func FindAndLoad(startDir string) *AgenticRC {
	paths := collectPaths(startDir)
	configs := loadConfigs(paths)
	return mergeConfigs(configs)
}

// FindLayers returns the .agenticrc.toml layers that FindAndLoad would merge,
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
		legacy := filepath.Join(dir, legacyRCFilename)
		if _, err := os.Stat(legacy); err == nil {
			_, _ = fmt.Fprintf(rcWarningWriter, "warning: found legacy %s at %s — rename to %s and convert to TOML syntax\n",
				legacyRCFilename, legacy, rcFilename)
		}

		candidate := filepath.Join(dir, rcFilename)
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
	resRun := &result.Run
	resBuild := &result.Build

	for _, rc := range configs {
		run := rc.Run

		if result.Namespace == "" {
			result.Namespace = rc.Namespace
		}

		if resRun.PidsLimit == "" {
			resRun.PidsLimit = run.PidsLimit
		}

		if resRun.CPUs == "" {
			resRun.CPUs = run.CPUs
		}

		if resRun.Memory == "" {
			resRun.Memory = run.Memory
		}
	}

	for i := len(configs) - 1; i >= 0; i-- {
		run := configs[i].Run
		build := configs[i].Build
		resRun.ExtraMounts = append(resRun.ExtraMounts, run.ExtraMounts...)
		resRun.Secrets = append(resRun.Secrets, run.Secrets...)
		resBuild.AptPackages = append(resBuild.AptPackages, build.AptPackages...)
	}

	return result
}

func loadRC(path string) (*AgenticRC, error) {
	rc := &AgenticRC{}
	md, err := toml.DecodeFile(path, rc)
	if err != nil {
		return nil, err
	}

	if keys := md.Undecoded(); len(keys) > 0 {
		return nil, fmt.Errorf("%s: unknown keys: %v", path, keys)
	}

	return rc, nil
}

// splitEnvValues splits a comma-separated value string and skips empty parts.
// Used for env var parsing where variable expansion is handled by the caller.
func splitEnvValues(value string) []string {
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
