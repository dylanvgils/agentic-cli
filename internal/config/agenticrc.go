// Package config provides project configuration loaded from .agenticrc.toml files.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

const rcFilename = ".agenticrc.toml"

// DefaultNamespace is the image namespace used when neither a flag nor .agenticrc.toml sets one.
const DefaultNamespace = "agentic"

// ModeEnforce and ModeMonitor are the only valid RCProxy.Mode values, besides unset "" (treated as ModeEnforce).
const (
	ModeEnforce = "enforce"
	ModeMonitor = "monitor"
)

// RCBuild holds build-time settings from a .agenticrc.toml file.
type RCBuild struct {
	Bases          []string          `toml:"bases"`
	AptPackages    []string          `toml:"apt_packages"`
	Versions       map[string]string `toml:"versions"`
	CustomInstalls []RCCustomInstall `toml:"custom_installs"`
}

// RCRun holds runtime settings from a .agenticrc.toml file.
type RCRun struct {
	Mounts []string `toml:"mounts"`
	Env    []string `toml:"env"`
	// Args are extra args per tool name, passed before the args given on the command line
	Args         map[string][]string `toml:"args"`
	Runtime      string              `toml:"runtime"`
	Proxy        RCProxy             `toml:"proxy"`
	Dind         RCDind              `toml:"dind"`
	Instructions RCInstructions      `toml:"instructions"`
	// Tool limits
	RCLimits
	// CheckUpdates is a pointer so an inner config can explicitly disable a check an outer one enabled; nil or true means it runs.
	CheckUpdates *bool `toml:"check_updates"`
}

// RCInstructions holds environment-instructions settings from a .agenticrc.toml file.
type RCInstructions struct {
	// Enabled is a pointer so an inner config can explicitly disable a block enabled by an outer one.
	Enabled *bool `toml:"enabled"`
	// Custom is free text appended after the generated sections; layers concatenate rather than override.
	Custom string `toml:"custom"`
}

// RCProxy holds egress-proxy settings from a .agenticrc.toml file. Enabled is a pointer so an inner config can explicitly disable a proxy an outer one enabled.
type RCProxy struct {
	Enabled      *bool    `toml:"enabled"`
	AllowedHosts []string `toml:"allowed_hosts"`
	// Mode selects how the proxy enforces AllowedHosts: "enforce" (default) blocks disallowed hosts, "monitor" only logs the verdict.
	Mode        string         `toml:"mode"`
	Credentials []RCCredential `toml:"credentials"`
	// AllowedPaths allows only some paths on hosts not in AllowedHosts; the proxy decrypts them to check each request
	AllowedPaths []RCAllowedPath `toml:"allowed_paths"`
	// Integrations route tools like Maven through the proxy
	Integrations []string `toml:"integrations"`
}

// RCAllowedPath allows requests to Host whose path matches one of Paths (see MatchPathPattern).
type RCAllowedPath struct {
	Host  string   `toml:"host"`
	Paths []string `toml:"paths"`
}

// RCCredential declares a secret swapped in for a placeholder the tool sends in a header; set either Preset or Hosts+Header.
type RCCredential struct {
	Preset string   `toml:"preset"`
	Hosts  []string `toml:"hosts"`
	Header string   `toml:"header"`
	// Env lists tool env vars set to a placeholder so tools that require a key still start
	Env []string `toml:"env"`
	// Secret is where the value is read from, e.g. a file path
	Secret string `toml:"secret"`
}

// RCDind holds Docker-in-Docker sidecar settings from a .agenticrc.toml file.
type RCDind struct {
	// Pointer so an inner config can disable what an outer one enabled
	Enabled *bool `toml:"enabled"`
	// Sidecar limits; empty inherits the tool's
	RCLimits
}

// RCLimits holds the pids_limit, cpus and memory keys shared by [run] and [run.dind].
type RCLimits struct {
	PidsLimit string `toml:"pids_limit"`
	CPUs      string `toml:"cpus"`
	Memory    string `toml:"memory"`
}

// RCMarketplace declares one git-based plugin marketplace to sync and mount into tool containers.
type RCMarketplace struct {
	Name string `toml:"name" json:"name"`
	URL  string `toml:"url" json:"url"`
	// Tools restricts which tools this marketplace is mounted into; empty means every tool.
	Tools []string `toml:"tools" json:"tools,omitempty"`
}

// RCCustomInstall declares one non-apt tool to install via arbitrary shell commands, applied unconditionally at build time (no --<name> gate).
type RCCustomInstall struct {
	Name string   `toml:"name" json:"name"`
	Run  []string `toml:"run" json:"run"`
}

// AgenticRC holds the parsed contents of a .agenticrc.toml project config file.
type AgenticRC struct {
	Root          bool            `toml:"root"`
	Namespace     string          `toml:"namespace"`
	DockerContext string          `toml:"docker_context"`
	Build         RCBuild         `toml:"build"`
	Run           RCRun           `toml:"run"`
	Marketplaces  []RCMarketplace `toml:"marketplaces"`
}

// RCLayer pairs a parsed .agenticrc.toml with the path it was loaded from.
type RCLayer struct {
	Path string
	RC   *AgenticRC
}

// FindAndLoadFromCwd loads config starting from the current working directory.
func FindAndLoadFromCwd() (*AgenticRC, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return FindAndLoad(cwd)
}

// MarketplacesFor returns the marketplace entries applicable to tool.
func MarketplacesFor(rc *AgenticRC, tool string) []RCMarketplace {
	var result []RCMarketplace
	for _, marketplace := range rc.Marketplaces {
		if len(marketplace.Tools) == 0 || slices.Contains(marketplace.Tools, tool) {
			result = append(result, marketplace)
		}
	}
	return result
}

// FindAndLoad walks up from startDir merging .agenticrc.toml layers (stopping at root=true): scalar keys take the innermost value, list keys accumulate outermost-first.
func FindAndLoad(startDir string) (*AgenticRC, error) {
	layers, err := FindLayers(startDir)
	if err != nil {
		return nil, err
	}

	return Merge(layers)
}

// FindLayers returns the .agenticrc.toml layers that FindAndLoad would merge, ordered outermost-to-innermost, each paired with its source path.
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

// SplitEnvValues splits a comma-separated value string and skips empty parts.
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

func loadRC(path string) (*AgenticRC, error) {
	rc := &AgenticRC{}
	md, err := toml.DecodeFile(path, rc)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	if keys := md.Undecoded(); len(keys) > 0 {
		return nil, fmt.Errorf("%s: unknown keys: %v", path, keys)
	}

	if err := validateRC(rc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return rc, nil
}
