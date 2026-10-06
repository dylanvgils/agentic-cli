// Package run builds the docker.RunSpec for `agentic run`, syncing marketplaces and assembling resolved volumes, secrets, env vars, and resource limits.
package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/credentials"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
)

// Service builds RunSpecs and run-time instructions for `agentic run`.
type Service struct {
	docker Docker
}

// Target identifies the tool and image a RunSpec is being built for.
type Target struct {
	ToolName       string
	ImageName      string
	SkipEntrypoint bool
}

// Input carries the flag/env-derived values Build needs.
type Input struct {
	ToolHome       string
	Volumes        []string
	Secrets        []string
	ReadOnlyMounts []string
	Env            []string
	Limits         docker.ResourceLimits
	DryRun         bool
	Registry       string
	ProxyMode      docker.ProxyMode
	DindEnabled    bool
	// Sidecar limit flags; empty falls back to config
	DindLimits docker.ResourceLimits
	// Credentials are the approved proxy credentials, from ResolveCredentials
	Credentials []credentials.Resolved
	// InstructionsMount is the mount spec for this run's instructions snapshot, empty when disabled.
	InstructionsMount string
}

// New returns a Service that talks to Docker through d.
func New(d Docker) *Service {
	return &Service{docker: d}
}

// Build assembles the docker.RunSpec for target, syncing marketplaces and ensuring the named volumes/network it depends on exist.
func (s *Service) Build(target Target, in Input, toolConfig tools.ToolConfig, rc *config.AgenticRC) (docker.RunSpec, error) {
	containerHome := s.docker.ResolveContainerHome(target.ImageName)

	marketplaceMounts, marketplaceNames, err := syncToolMarketplaces(in.ToolHome, target.ToolName, toolConfig, rc)
	if err != nil {
		return docker.RunSpec{}, err
	}

	volumes := resolve.Volumes(toolConfig.Runtime.Mounts(), in.Volumes, rc)
	volumes = append(volumes, marketplaceMounts...)
	if in.InstructionsMount != "" {
		volumes = append(volumes, in.InstructionsMount)
	}
	// Must stay last: Docker lets the last --volume flag for a path win.
	volumes = append(volumes, readOnlyMountSpecs(resolve.ReadOnlyMounts(in.ReadOnlyMounts, rc))...)
	secrets := resolve.Secrets(in.Secrets, rc)
	env := resolve.Env(in.Env, rc)
	limits := resolve.ResourceLimitsFor(in.Limits, rc)
	dindLimits := resolve.DindResourceLimitsFor(in.DindLimits, rc, limits)

	if err := validateEnv(env, in.ProxyMode.Enabled(), in.DindEnabled); err != nil {
		return docker.RunSpec{}, err
	}

	placeholders, err := credentialSetup(in, volumes, secrets, env, containerHome)
	if err != nil {
		return docker.RunSpec{}, err
	}
	env = append(env, placeholders...)

	if len(in.Credentials) > 0 {
		if err := s.checkProxyTrust(target); err != nil {
			return docker.RunSpec{}, err
		}
	}

	// Tells entrypoint.sh exactly which names to register instead of globbing.
	if len(marketplaceNames) > 0 {
		env = append(env, "AGENTIC_MARKETPLACES="+strings.Join(marketplaceNames, ","))
	}

	if err := s.docker.EnsureNamedVolumes(volumes, in.ToolHome, containerHome, tools.BusyboxImageFor(in.Registry)); err != nil {
		return docker.RunSpec{}, err
	}

	// In proxy or dind mode the tool container attaches to a per-run network
	// instead of agentic-net; startProxy ensures agentic-net itself for the
	// sidecar's egress connection, so skip the redundant check here.
	if !in.ProxyMode.Enabled() && !in.DindEnabled {
		if err := s.docker.EnsureNetwork(); err != nil {
			return docker.RunSpec{}, err
		}
	}

	logDir, err := proxyLogDir(in.ToolHome, in.ProxyMode.Enabled())
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
		WithLimits(limits).
		WithDryRun(in.DryRun).
		WithProxy(docker.ProxySpec{
			Mode:        in.ProxyMode,
			Image:       tools.ProxyImage,
			Allow:       resolve.ProxyAllowList(toolConfig.Runtime.AllowedHosts, in.DindEnabled, rc),
			LogDir:      logDir,
			Credentials: proxyCredentials(in.Credentials),
		}).
		WithDind(docker.DindSpec{Enabled: in.DindEnabled, Image: tools.DindImage, Limits: dindLimits}).
		Build()

	return rs, nil
}

// BuildWithInstructions wraps Build with this run's instructions snapshot mounted in; the returned cleanup func must always be deferred, even on error.
func (s *Service) BuildWithInstructions(target Target, in Input, toolConfig tools.ToolConfig, rc *config.AgenticRC) (docker.RunSpec, func(), error) {
	content, err := s.BuildInstructions(target, in, toolConfig, rc)
	if err != nil {
		return docker.RunSpec{}, func() {}, fmt.Errorf("build instructions for %s: %w", target.ToolName, err)
	}

	snapshot, err := s.PrepareInstructions(in.ToolHome, toolConfig, content)
	if err != nil {
		return docker.RunSpec{}, func() {}, fmt.Errorf("prepare instructions for %s: %w", target.ToolName, err)
	}

	in.InstructionsMount = snapshot.MountSpec

	rs, err := s.Build(target, in, toolConfig, rc)
	if err != nil {
		snapshot.Cleanup()
		return docker.RunSpec{}, func() {}, err
	}

	return rs, snapshot.Cleanup, nil
}

// ToolNeedsMarketplaceSync reports whether tool supports marketplace mounting and has at least one marketplace configured.
func (s *Service) ToolNeedsMarketplaceSync(toolConfig tools.ToolConfig, rc *config.AgenticRC, tool string) bool {
	return needsMarketplaceSync(toolConfig, rc, tool)
}

// needsMarketplaceSync backs ToolNeedsMarketplaceSync for callers without a Service.
func needsMarketplaceSync(toolConfig tools.ToolConfig, rc *config.AgenticRC, tool string) bool {
	if toolConfig.Runtime.MarketplaceMount == nil {
		return false
	}
	return len(config.MarketplacesFor(rc, tool)) > 0
}

// syncToolMarketplaces syncs tool's configured marketplaces and returns each mount spec plus name.
func syncToolMarketplaces(toolHome, tool string, toolConfig tools.ToolConfig, rc *config.AgenticRC) (mounts, names []string, err error) {
	if !needsMarketplaceSync(toolConfig, rc, tool) {
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
			logging.Warnf("marketplace %q: %v; using existing clone", r.Entry.Name, r.Warning)
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
		logging.Warnf("could not record marketplace usage: %v", err)
		return
	}

	if err := RecordMarketplaceUsage(baseDir, results, cwd); err != nil {
		logging.Warnf("could not record marketplace usage: %v", err)
	}
}

// readOnlyMountSpecs converts each entry into a forced-:ro volume spec; a no-colon entry expands to "$PWD/<path>:/workspace/<path>".
func readOnlyMountSpecs(specs []string) []string {
	out := make([]string, 0, len(specs))
	for _, spec := range specs {
		host, container := splitReadOnlyMountSpec(spec)
		out = append(out, mount.VolumeMount(host, container, mount.VolumeOptions{ReadOnly: true}))
	}
	return out
}

// splitReadOnlyMountSpec splits a read-only mount entry, expanding the no-colon workspace-relative shorthand.
func splitReadOnlyMountSpec(spec string) (host, container string) {
	if !strings.Contains(spec, ":") {
		return "$PWD/" + spec, mount.WorkspaceContainerPath + "/" + spec
	}

	host, container, _ = mount.SplitHostContainer(spec)
	container = strings.TrimSuffix(strings.TrimSuffix(container, ":ro"), ":rw")
	return host, container
}

// validateEnv rejects entries that override an env var agentic manages.
func validateEnv(entries []string, proxyEnabled, dindEnabled bool) error {
	for _, entry := range entries {
		key, _, _ := strings.Cut(entry, "=")
		if docker.IsReservedEnvName(key, proxyEnabled) || (dindEnabled && docker.IsReservedDindEnvName(key)) {
			return fmt.Errorf("--env: %q is managed by agentic and cannot be overridden", key)
		}
	}

	return nil
}
