// Package config provides project configuration loaded from .agenticrc.toml files.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// RCBuild holds build-time settings from a .agenticrc.toml file.
type RCBuild struct {
	Bases       []string          `toml:"bases"`
	AptPackages []string          `toml:"apt_packages"`
	Versions    map[string]string `toml:"versions"`
}

// RCRun holds runtime settings from a .agenticrc.toml file.
type RCRun struct {
	ExtraMounts []string `toml:"extra_mounts"`
	Secrets     []string `toml:"secrets"`
	Env         []string `toml:"env"`
	PidsLimit   string   `toml:"pids_limit"`
	CPUs        string   `toml:"cpus"`
	Memory      string   `toml:"memory"`
	Proxy       RCProxy  `toml:"proxy"`
}

// RCProxy holds egress-proxy settings from a .agenticrc.toml file. Enabled is a
// pointer so an inner config can explicitly disable a proxy enabled by an outer
// one (a plain false is indistinguishable from "unset").
type RCProxy struct {
	Enabled      *bool    `toml:"enabled"`
	AllowedHosts []string `toml:"allowed_hosts"`
}

// AgenticRC holds the parsed contents of a .agenticrc.toml project config file.
type AgenticRC struct {
	Root      bool    `toml:"root"`
	Namespace string  `toml:"namespace"`
	Build     RCBuild `toml:"build"`
	Run       RCRun   `toml:"run"`
}

// RCLayer pairs a parsed .agenticrc.toml with the path it was loaded from.
type RCLayer struct {
	Path string
	RC   *AgenticRC
}

const rcFilename = ".agenticrc.toml"

// FindAndLoadFromCwd loads config starting from the current working directory.
func FindAndLoadFromCwd() (*AgenticRC, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return FindAndLoad(cwd)
}

// AptPackages returns the merged apt packages from rc and the AGENTIC_APT_PACKAGES
// env var, RC values first, env var last.
func AptPackages(rc *AgenticRC) []string {
	envPkgs := SplitEnvValues(os.Getenv(EnvAptPackages))
	return append(rc.Build.AptPackages, envPkgs...)
}

// FindAndLoad walks up from startDir collecting all .agenticrc.toml files and
// merges them. Stops when a file with root=true is encountered. For scalar keys
// the innermost (child) value wins; list keys accumulate outermost-first.
// Returns an empty AgenticRC if no file is found.
func FindAndLoad(startDir string) (*AgenticRC, error) {
	paths := collectPaths(startDir)

	configs, err := loadConfigs(paths)
	if err != nil {
		return nil, err
	}

	return mergeConfigs(configs), nil
}

// FindLayers returns the .agenticrc.toml layers that FindAndLoad would merge,
// ordered outermost-to-innermost, each paired with its source path.
func FindLayers(startDir string) ([]RCLayer, error) {
	paths := collectPaths(startDir)
	var layers []RCLayer

	for _, path := range paths {
		rc, err := loadRC(path)
		if err != nil {
			return nil, err
		}

		layers = append(layers, RCLayer{Path: path, RC: rc})
		if rc.Root {
			break
		}
	}

	for i, j := 0, len(layers)-1; i < j; i, j = i+1, j-1 {
		layers[i], layers[j] = layers[j], layers[i]
	}

	return layers, nil
}

func collectPaths(startDir string) []string {
	var paths []string
	dir := startDir

	for {
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

func loadConfigs(paths []string) ([]*AgenticRC, error) {
	var configs []*AgenticRC

	for _, path := range paths {
		rc, err := loadRC(path)
		if err != nil {
			return nil, err
		}
		configs = append(configs, rc)

		if rc.Root {
			break
		}
	}

	return configs, nil
}

func mergeConfigs(configs []*AgenticRC) *AgenticRC {
	result := &AgenticRC{}
	result.Build.Versions = make(map[string]string)
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

		if resRun.Proxy.Enabled == nil {
			resRun.Proxy.Enabled = run.Proxy.Enabled
		}

		for key, val := range rc.Build.Versions {
			if _, exists := result.Build.Versions[key]; !exists {
				result.Build.Versions[key] = val
			}
		}
	}

	for i := len(configs) - 1; i >= 0; i-- {
		run := configs[i].Run
		build := configs[i].Build
		resRun.ExtraMounts = append(resRun.ExtraMounts, run.ExtraMounts...)
		resRun.Secrets = append(resRun.Secrets, run.Secrets...)
		resRun.Env = append(resRun.Env, run.Env...)
		resRun.Proxy.AllowedHosts = append(resRun.Proxy.AllowedHosts, run.Proxy.AllowedHosts...)
		resBuild.AptPackages = append(resBuild.AptPackages, build.AptPackages...)
		resBuild.Bases = append(resBuild.Bases, build.Bases...)
	}

	return result
}

func loadRC(path string) (*AgenticRC, error) {
	rc := &AgenticRC{}
	md, err := toml.DecodeFile(path, rc)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	if keys := md.Undecoded(); len(keys) > 0 {
		return nil, fmt.Errorf("%s: unknown keys: %v", path, keys)
	}

	return rc, nil
}

// SplitEnvValues splits a comma-separated value string and skips empty parts.
// Used for env var parsing where variable expansion is handled by the caller.
func SplitEnvValues(value string) []string {
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
