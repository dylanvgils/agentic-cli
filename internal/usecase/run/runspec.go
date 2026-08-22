// Package run builds the docker.RunSpec for `agentic run`, syncing
// configured marketplaces and merging config/flag/env sources for volumes,
// secrets, env vars, and resource limits.
package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// Target identifies the tool and image a RunSpec is being built for.
type Target struct {
	ToolName       string
	ImageName      string
	SkipEntrypoint bool
}

// Input carries the flag/env-derived values Build needs.
type Input struct {
	ToolHome     string
	Volumes      []string
	Secrets      []string
	Env          []string
	PidsLimit    string
	CPUs         string
	Memory       string
	DryRun       bool
	Registry     string
	ProxyEnabled bool
	ProxyMonitor bool
	// InstructionsMount is the mount spec for this run's instructions snapshot, empty when disabled.
	InstructionsMount string
}

type resourceLimits struct {
	pidsLimit string
	cpus      string
	memory    string
}

// Build assembles the docker.RunSpec for target, syncing marketplaces and
// ensuring the named volumes/network it depends on exist.
func Build(target Target, in Input, toolConfig tools.ToolConfig, rc *config.AgenticRC) (docker.RunSpec, error) {
	containerHome := docker.ResolveContainerHome(target.ImageName)

	marketplaceMounts, marketplaceNames, err := syncToolMarketplaces(in.ToolHome, target.ToolName, toolConfig, rc)
	if err != nil {
		return docker.RunSpec{}, err
	}

	volumes := collectVolumes(toolConfig.Runtime.Mounts(), in.Volumes, rc)
	volumes = append(volumes, marketplaceMounts...)
	if in.InstructionsMount != "" {
		volumes = append(volumes, in.InstructionsMount)
	}
	// Read-only sub-path overrides must stay last: Docker shadows an
	// overlapping bind-mount path with whichever --volume flag for it comes
	// last, so this is what actually enforces the sub-path override.
	volumes = append(volumes, readOnlyMountSpecs(rc.Run.ReadOnlyMounts)...)
	secrets := collectSecrets(in.Secrets, rc)
	env := collectEnv(in.Env, rc)
	limits := resolveResourceLimits(in.PidsLimit, in.CPUs, in.Memory, rc)

	if err := validateEnv(env, in.ProxyEnabled); err != nil {
		return docker.RunSpec{}, err
	}

	// Tells entrypoint.sh exactly which names to register instead of globbing.
	if len(marketplaceNames) > 0 {
		env = append(env, "AGENTIC_MARKETPLACES="+strings.Join(marketplaceNames, ","))
	}

	if err := EnsureNamedVolumes(volumes, in.ToolHome, containerHome, tools.BusyboxImageFor(in.Registry)); err != nil {
		return docker.RunSpec{}, err
	}

	// In proxy mode the tool container attaches to a per-run internal network
	// instead of agentic-net; startProxy ensures agentic-net itself for the
	// sidecar's egress connection, so skip the redundant check here.
	if !in.ProxyEnabled {
		if err := EnsureNetwork(); err != nil {
			return docker.RunSpec{}, err
		}
	}

	logDir, err := proxyLogDir(in.ToolHome, in.ProxyEnabled)
	if err != nil {
		return docker.RunSpec{}, err
	}

	rs := docker.NewRunSpec(target.ImageName).
		WithToolHome(in.ToolHome).
		WithContainerHome(containerHome).
		WithVolumes(volumes...).
		WithSecrets(secrets...).
		WithEnv(env...).
		WithSkipEntrypoint(target.SkipEntrypoint).
		WithTmpfsMounts(toolConfig.Runtime.TmpfsMounts()...).
		WithPidsLimit(limits.pidsLimit).
		WithCPUs(limits.cpus).
		WithMemory(limits.memory).
		WithDryRun(in.DryRun).
		WithProxy(in.ProxyEnabled, tools.ProxyImage, proxyAllowList(toolConfig, rc), logDir, in.ProxyMonitor).
		Build()

	return rs, nil
}

// BuildWithInstructions wraps Build with this run's instructions snapshot mounted in; the returned cleanup func must always be deferred, even on error.
func BuildWithInstructions(target Target, in Input, toolConfig tools.ToolConfig, rc *config.AgenticRC) (docker.RunSpec, func(), error) {
	content, err := BuildInstructions(target, in, toolConfig, rc)
	if err != nil {
		return docker.RunSpec{}, func() {}, fmt.Errorf("build instructions for %s: %w", target.ToolName, err)
	}

	snapshot, err := PrepareInstructions(in.ToolHome, toolConfig, content)
	if err != nil {
		return docker.RunSpec{}, func() {}, fmt.Errorf("prepare instructions for %s: %w", target.ToolName, err)
	}

	in.InstructionsMount = snapshot.MountSpec

	rs, err := Build(target, in, toolConfig, rc)
	if err != nil {
		snapshot.Cleanup()
		return docker.RunSpec{}, func() {}, err
	}

	return rs, snapshot.Cleanup, nil
}

// ToolNeedsMarketplaceSync reports whether tool has marketplace mounting
// support and at least one marketplace configured for it.
func ToolNeedsMarketplaceSync(toolConfig tools.ToolConfig, rc *config.AgenticRC, tool string) bool {
	if toolConfig.Runtime.MarketplaceMount == nil {
		return false
	}
	return len(config.MarketplacesFor(rc, tool)) > 0
}

// syncToolMarketplaces syncs tool's configured marketplaces and returns each mount spec plus name.
func syncToolMarketplaces(toolHome, tool string, toolConfig tools.ToolConfig, rc *config.AgenticRC) (mounts, names []string, err error) {
	if !ToolNeedsMarketplaceSync(toolConfig, rc, tool) {
		return nil, nil, nil
	}

	entries := config.MarketplacesFor(rc, tool)
	mpEntries := make([]marketplace.Entry, len(entries))
	for i, e := range entries {
		mpEntries[i] = marketplace.Entry{Name: e.Name, URL: e.URL}
	}

	baseDir := filepath.Join(toolHome, marketplace.MarketplacesDirName)
	results, err := SyncMarketplaces(mpEntries, func(e marketplace.Entry) string {
		return filepath.Join(baseDir, marketplace.CloneDirName(e.URL))
	})
	if err != nil {
		return nil, nil, fmt.Errorf("sync marketplaces for %s: %w", tool, err)
	}

	mounts = make([]string, len(results))
	names = make([]string, len(results))
	for i, r := range results {
		if r.Stale {
			fmt.Fprintf(os.Stderr, "warning: marketplace %q: %v; using existing clone\n", r.Entry.Name, r.Warning)
		}
		mounts[i] = toolConfig.Runtime.MarketplaceMount(r.Entry.Name, r.Entry.URL)
		names[i] = r.Entry.Name
	}

	recordMarketplaceUsage(baseDir, results)

	return mounts, names, nil
}

// recordMarketplaceUsage records cwd against each result for `marketplaces prune`. Best-effort.
func recordMarketplaceUsage(baseDir string, results []marketplace.Result) {
	if len(results) == 0 {
		return
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not record marketplace usage: %v\n", err)
		return
	}

	if err := RecordMarketplaceUsage(baseDir, results, cwd); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not record marketplace usage: %v\n", err)
	}
}

func collectVolumes(toolMounts []string, extra []string, rc *config.AgenticRC) []string {
	volumes := append([]string{}, toolMounts...)

	if env := os.Getenv("AGENTIC_EXTRA_MOUNTS"); env != "" {
		for m := range strings.SplitSeq(env, ",") {
			if m != "" {
				volumes = append(volumes, m)
			}
		}
	}
	volumes = append(volumes, extra...)
	volumes = append(volumes, rc.Run.ExtraMounts...)

	return volumes
}

// readOnlyMountSpecs converts each "host:container" entry into a forced-:ro
// volume spec, stripping any user-supplied :ro/:rw suffix first (mirrors
// buildSecretArgs's precedent of forcing :ro at assembly time regardless of
// the source spec). Callers must append the result last to the final volumes
// list: Docker shadows an overlapping bind-mount path with whichever
// --volume flag for it appears last, so this sub-path override only takes
// effect when it is ordered after the parent read-write mount.
func readOnlyMountSpecs(specs []string) []string {
	out := make([]string, 0, len(specs))
	for _, spec := range specs {
		host := mount.HostPart(spec)
		container, _ := strings.CutPrefix(spec, host+":")
		container = strings.TrimSuffix(strings.TrimSuffix(container, ":ro"), ":rw")
		out = append(out, mount.VolumeMount(host, container, mount.VolumeOptions{ReadOnly: true}))
	}
	return out
}

func collectSecrets(flags []string, rc *config.AgenticRC) []string {
	var secrets []string

	if env := os.Getenv("AGENTIC_SECRETS"); env != "" {
		for s := range strings.SplitSeq(env, ",") {
			if s != "" {
				secrets = append(secrets, s)
			}
		}
	}
	secrets = append(secrets, flags...)
	secrets = append(secrets, rc.Run.Secrets...)

	return secrets
}

func collectEnv(flags []string, rc *config.AgenticRC) []string {
	env := append([]string{}, rc.Run.Env...)
	env = append(env, flags...)

	return env
}

// validateEnv rejects entries that target an env var agentic already manages
// (proxy injection when proxyEnabled, mount placeholders always).
func validateEnv(entries []string, proxyEnabled bool) error {
	for _, entry := range entries {
		key, _, _ := strings.Cut(entry, "=")
		if docker.IsReservedEnvName(key, proxyEnabled) {
			return fmt.Errorf("--env: %q is managed by agentic and cannot be overridden", key)
		}
	}

	return nil
}

// resolveResourceLimits resolves each limit through flag, then rc, then env var, then hardcoded default.
func resolveResourceLimits(pidsLimit, cpus, memory string, rc *config.AgenticRC) resourceLimits {
	run := rc.Run
	if pidsLimit == "" {
		pidsLimit = run.PidsLimit
	}
	if cpus == "" {
		cpus = run.CPUs
	}
	if memory == "" {
		memory = run.Memory
	}

	return resourceLimits{
		pidsLimit: resolveLimit(pidsLimit, config.EnvPidsLimit, docker.DefaultPidsLimit),
		cpus:      resolveLimit(cpus, config.EnvCPUs, docker.DefaultCPUs),
		memory:    resolveLimit(memory, config.EnvMemory, docker.DefaultMemory),
	}
}

// resolveLimit returns val if non-empty, then the env var, then fallback.
func resolveLimit(val, envKey, fallback string) string {
	if val != "" {
		return val
	}
	if env := os.Getenv(envKey); env != "" {
		return env
	}
	return fallback
}
