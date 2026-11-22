// Package agenticrc loads, merges and validates .agenticrc.toml layers.
package agenticrc

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/BurntSushi/toml"
)

const rcFilename = ".agenticrc.toml"

// DefaultNamespace is the image namespace used when neither a flag nor .agenticrc.toml sets one.
const DefaultNamespace = "agentic"

// ModeEnforce and ModeMonitor are the only valid Proxy.Mode values, besides unset "" (treated as ModeEnforce).
const (
	ModeEnforce = "enforce"
	ModeMonitor = "monitor"
)

// BackendNative and BackendAgentic are the only valid Proxy.Backend values, besides unset "" (the runtime's default).
const (
	BackendNative  = "native"
	BackendAgentic = "agentic"
)

// Build holds build-time settings from a .agenticrc.toml file.
type Build struct {
	Bases          []string          `toml:"bases"`
	AptPackages    []string          `toml:"apt_packages"`
	Versions       map[string]string `toml:"versions"`
	CustomInstalls []CustomInstall   `toml:"custom_installs"`
}

// Run holds runtime settings from a .agenticrc.toml file.
type Run struct {
	Mounts []string `toml:"mounts"`
	Env    []string `toml:"env"`
	Ports  []string `toml:"ports"`
	// Args are extra args per tool name, passed before the args given on the command line
	Args         map[string][]string `toml:"args"`
	Runtime      string              `toml:"runtime"`
	Proxy        Proxy               `toml:"proxy"`
	Dind         Dind                `toml:"dind"`
	Instructions Instructions        `toml:"instructions"`
	// Tool limits
	Limits
	// CheckUpdates is a pointer so an inner config can explicitly disable a check an outer one enabled; nil or true means it runs.
	CheckUpdates *bool `toml:"check_updates"`
}

// Instructions holds environment-instructions settings from a .agenticrc.toml file.
type Instructions struct {
	// Enabled is a pointer so an inner config can explicitly disable a block enabled by an outer one.
	Enabled *bool `toml:"enabled"`
	// Custom is free text appended after the generated sections; layers concatenate rather than override.
	Custom string `toml:"custom"`
}

// Proxy holds egress-proxy settings from a .agenticrc.toml file. Enabled is a pointer so an inner config can explicitly disable a proxy an outer one enabled.
type Proxy struct {
	Enabled      *bool    `toml:"enabled"`
	AllowedHosts []string `toml:"allowed_hosts"`
	// Mode selects how the proxy enforces AllowedHosts: "enforce" (default) blocks disallowed hosts, "monitor" only logs the verdict.
	Mode string `toml:"mode"`
	// Backend picks what enforces AllowedHosts on microsandbox: "native" (default) msb rules or "agentic" agentic-proxy.
	Backend     string       `toml:"backend"`
	Credentials []Credential `toml:"credentials"`
	// AllowedPaths allows only some paths on hosts not in AllowedHosts; the proxy decrypts them to check each request
	AllowedPaths []AllowedPath `toml:"allowed_paths"`
	// Integrations route tools like Maven through the proxy
	Integrations []string `toml:"integrations"`
}

// AllowedPath allows requests to Host whose path matches one of Paths (see pathpattern.Match).
type AllowedPath struct {
	Host  string   `toml:"host"`
	Paths []string `toml:"paths"`
}

// Credential declares a secret swapped in for a placeholder the tool sends in a header; set either Preset or Hosts+Header.
type Credential struct {
	Preset string   `toml:"preset"`
	Hosts  []string `toml:"hosts"`
	Header string   `toml:"header"`
	// Env lists tool env vars set to a placeholder so tools that require a key still start
	Env []string `toml:"env"`
	// Secret is where the value is read from, e.g. a file path
	Secret string `toml:"secret"`
}

// Dind holds Docker-in-Docker sidecar settings from a .agenticrc.toml file.
type Dind struct {
	// Pointer so an inner config can disable what an outer one enabled
	Enabled *bool `toml:"enabled"`
	// Sidecar limits; empty inherits the tool's
	Limits
}

// Limits holds the pids_limit, cpus and memory keys shared by [run] and [run.dind].
type Limits struct {
	PidsLimit string `toml:"pids_limit"`
	CPUs      string `toml:"cpus"`
	Memory    string `toml:"memory"`
}

// Marketplace declares one git-based plugin marketplace to sync and mount into tool containers.
type Marketplace struct {
	Name string `toml:"name" json:"name"`
	URL  string `toml:"url" json:"url"`
	// Tools restricts which tools this marketplace is mounted into; empty means every tool.
	Tools []string `toml:"tools" json:"tools,omitempty"`
}

// CustomInstall declares one non-apt tool to install via arbitrary shell commands, applied unconditionally at build time (no --<name> gate).
type CustomInstall struct {
	Name string   `toml:"name" json:"name"`
	Run  []string `toml:"run" json:"run"`
}

// Config holds the parsed contents of a .agenticrc.toml project config file.
type Config struct {
	Root          bool          `toml:"root"`
	Namespace     string        `toml:"namespace"`
	DockerContext string        `toml:"docker_context"`
	Build         Build         `toml:"build"`
	Run           Run           `toml:"run"`
	Marketplaces  []Marketplace `toml:"marketplaces"`
}

// Layer pairs a parsed .agenticrc.toml with the path it was loaded from.
type Layer struct {
	Path   string
	Config *Config
}

// Project is a work dir's .agenticrc.toml layers and their merge, loaded once so every step of a command sees the same config.
type Project struct {
	WorkDir string
	Layers  []Layer
	Config  *Config
}

// MarketplacesFor returns the marketplace entries applicable to tool.
func (c *Config) MarketplacesFor(tool string) []Marketplace {
	var result []Marketplace
	for _, marketplace := range c.Marketplaces {
		if len(marketplace.Tools) == 0 || slices.Contains(marketplace.Tools, tool) {
			result = append(result, marketplace)
		}
	}
	return result
}

// Load walks up from workDir merging .agenticrc.toml layers (stopping at root=true): scalar keys take the innermost
// value, list keys accumulate outermost-first. On error only WorkDir is set.
func Load(workDir string) (Project, error) {
	layers, err := findLayers(workDir)
	if err != nil {
		return Project{WorkDir: workDir}, err
	}

	rc, err := merge(layers)
	if err != nil {
		return Project{WorkDir: workDir}, err
	}

	return Project{
		WorkDir: workDir,
		Layers:  layers,
		Config:  rc,
	}, nil
}

// findLayers returns the .agenticrc.toml layers that Load merges, ordered outermost-to-innermost, each paired with its source path.
func findLayers(startDir string) ([]Layer, error) {
	paths := collectPaths(startDir)
	var layers []Layer

	for _, path := range paths {
		rc, err := loadRC(path)
		if err != nil {
			return nil, err
		}

		layers = append(layers, Layer{Path: path, Config: rc})
		if rc.Root {
			break
		}
	}

	slices.Reverse(layers)
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

func loadRC(path string) (*Config, error) {
	rc := &Config{}
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
