// Package config provides project configuration loaded from .agenticrc.toml files.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

const rcFilename = ".agenticrc.toml"

// DefaultNamespace is the image namespace used when neither a flag nor .agenticrc.toml sets one.
const DefaultNamespace = "agentic"

// validRCEntryName matches safe path-segment/identifier characters, used for both marketplace and custom-install names.
var validRCEntryName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

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
	ExtraMounts    []string       `toml:"extra_mounts"`
	ReadOnlyMounts []string       `toml:"read_only_mounts"`
	Secrets        []string       `toml:"secrets"`
	Env            []string       `toml:"env"`
	Proxy          RCProxy        `toml:"proxy"`
	Dind           RCDind         `toml:"dind"`
	Instructions   RCInstructions `toml:"instructions"`
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
}

// RCCredential declares a secret the proxy injects as a header; set either Preset or Hosts+Header.
type RCCredential struct {
	Preset string   `toml:"preset"`
	Hosts  []string `toml:"hosts"`
	Header string   `toml:"header"`
	// Format wraps the secret, e.g. "Bearer %s"; empty means the bare secret
	Format string `toml:"format"`
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
	Name string `toml:"name"`
	URL  string `toml:"url"`
	// Tools restricts which tools this marketplace is mounted into; empty means every tool.
	Tools []string `toml:"tools"`
}

// RCCustomInstall declares one non-apt tool to install via arbitrary shell commands, applied unconditionally at build time (no --<name> gate).
type RCCustomInstall struct {
	Name string   `toml:"name"`
	Run  []string `toml:"run"`
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
	paths := collectPaths(startDir)

	configs, err := loadConfigs(paths)
	if err != nil {
		return nil, err
	}

	merged := mergeConfigs(configs)
	if err := validateUniqueCustomInstalls(merged.Build.CustomInstalls); err != nil {
		return nil, err
	}

	return merged, nil
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

		if result.DockerContext == "" {
			result.DockerContext = rc.DockerContext
		}

		resRun.RCLimits = mergeLimits(resRun.RCLimits, run.RCLimits)

		if resRun.Proxy.Enabled == nil {
			resRun.Proxy.Enabled = run.Proxy.Enabled
		}

		if resRun.Proxy.Mode == "" {
			resRun.Proxy.Mode = run.Proxy.Mode
		}

		if resRun.Dind.Enabled == nil {
			resRun.Dind.Enabled = run.Dind.Enabled
		}

		resRun.Dind.RCLimits = mergeLimits(resRun.Dind.RCLimits, run.Dind.RCLimits)

		if resRun.Instructions.Enabled == nil {
			resRun.Instructions.Enabled = run.Instructions.Enabled
		}

		if resRun.CheckUpdates == nil {
			resRun.CheckUpdates = run.CheckUpdates
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
		resRun.ReadOnlyMounts = append(resRun.ReadOnlyMounts, run.ReadOnlyMounts...)
		resRun.Secrets = append(resRun.Secrets, run.Secrets...)
		resRun.Env = append(resRun.Env, run.Env...)
		resRun.Proxy.AllowedHosts = append(resRun.Proxy.AllowedHosts, run.Proxy.AllowedHosts...)
		resRun.Proxy.Credentials = append(resRun.Proxy.Credentials, run.Proxy.Credentials...)
		resRun.Instructions.Custom = appendInstructions(resRun.Instructions.Custom, run.Instructions.Custom)
		resBuild.AptPackages = append(resBuild.AptPackages, build.AptPackages...)
		resBuild.Bases = append(resBuild.Bases, build.Bases...)
		resBuild.CustomInstalls = append(resBuild.CustomInstalls, build.CustomInstalls...)
		result.Marketplaces = append(result.Marketplaces, configs[i].Marketplaces...)
	}

	return result
}

// appendInstructions joins two layers' custom-instructions text, skipping an empty side.
func appendInstructions(existing, next string) string {
	if next == "" {
		return existing
	}
	if existing == "" {
		return next
	}
	return existing + "\n\n" + next
}

// mergeLimits returns limits with each empty value filled from parent.
func mergeLimits(limits, parent RCLimits) RCLimits {
	if limits.PidsLimit == "" {
		limits.PidsLimit = parent.PidsLimit
	}

	if limits.CPUs == "" {
		limits.CPUs = parent.CPUs
	}

	if limits.Memory == "" {
		limits.Memory = parent.Memory
	}

	return limits
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

	mode := rc.Run.Proxy.Mode
	if mode != "" && mode != ModeEnforce && mode != ModeMonitor {
		return nil, fmt.Errorf("%s: invalid [run.proxy] mode %q: must be %q or %q", path, mode, ModeEnforce, ModeMonitor)
	}

	for i, cred := range rc.Run.Proxy.Credentials {
		if err := validateCredential(cred); err != nil {
			return nil, fmt.Errorf("%s: [[run.proxy.credentials]] entry %d: %w", path, i+1, err)
		}
	}

	for _, marketplace := range rc.Marketplaces {
		if err := validateMarketplace(marketplace); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}

	seen := make(map[string]bool, len(rc.Build.CustomInstalls))
	for _, install := range rc.Build.CustomInstalls {
		if err := validateCustomInstall(install); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if seen[install.Name] {
			return nil, fmt.Errorf("%s: custom install %q: name must be unique", path, install.Name)
		}
		seen[install.Name] = true
	}

	return rc, nil
}

// validateCredential checks the entry's shape; hosts, headers and presets are checked when it is resolved.
func validateCredential(cred RCCredential) error {
	if cred.Secret == "" {
		return fmt.Errorf("secret must not be empty")
	}

	custom := len(cred.Hosts) > 0 || cred.Header != "" || cred.Format != ""
	if cred.Preset != "" && custom {
		return fmt.Errorf("preset %q cannot be combined with hosts, header or format", cred.Preset)
	}
	if cred.Preset == "" && (len(cred.Hosts) == 0 || cred.Header == "") {
		return fmt.Errorf("set either preset or both hosts and header")
	}
	return nil
}

func validateMarketplace(marketplace RCMarketplace) error {
	if marketplace.Name == "" {
		return fmt.Errorf("marketplace: name must not be empty")
	}
	if !validRCEntryName.MatchString(marketplace.Name) {
		return fmt.Errorf("marketplace %q: name must match %s", marketplace.Name, validRCEntryName.String())
	}
	if marketplace.URL == "" {
		return fmt.Errorf("marketplace %q: url must not be empty", marketplace.Name)
	}
	return nil
}

func validateCustomInstall(install RCCustomInstall) error {
	if install.Name == "" {
		return fmt.Errorf("custom install: name must not be empty")
	}
	if !validRCEntryName.MatchString(install.Name) {
		return fmt.Errorf("custom install %q: name must match %s", install.Name, validRCEntryName.String())
	}
	if len(install.Run) == 0 {
		return fmt.Errorf("custom install %q: run must not be empty", install.Name)
	}
	return nil
}

func validateUniqueCustomInstalls(installs []RCCustomInstall) error {
	seen := make(map[string]bool, len(installs))
	for _, install := range installs {
		if seen[install.Name] {
			return fmt.Errorf("custom install %q: name must be unique across .agenticrc.toml layers", install.Name)
		}
		seen[install.Name] = true
	}
	return nil
}
