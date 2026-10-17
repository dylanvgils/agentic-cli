// Package run builds the sandbox.RunSpec for `agentic run`, syncing marketplaces and assembling resolved volumes, secrets, env vars, and resource limits.
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
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
)

// Service builds RunSpecs and run-time instructions for `agentic run`.
type Service struct {
	docker       Docker
	microsandbox Microsandbox
	// syncMarketplaces clones or updates marketplaces with git; tests swap it so they need no network
	syncMarketplaces func(entries []marketplace.Entry, dirFor func(marketplace.Entry) string) ([]marketplace.Result, error)
}

// Target identifies the tool and image a RunSpec is being built for.
type Target struct {
	ToolName       string
	ImageName      string
	SkipEntrypoint bool
}

// Input carries the flag/env-derived values Build needs.
type Input struct {
	ToolHome    string
	Volumes     []string
	Secrets     []string
	Env         []string
	Limits      sandbox.ResourceLimits
	DryRun      bool
	Registry    string
	ProxyMode   sandbox.ProxyMode
	DindEnabled bool
	// Sidecar limit flags; empty falls back to config
	DindLimits sandbox.ResourceLimits
	// Runtime is where the run executes; empty means Docker
	Runtime Runtime
	// Credentials are the approved proxy credentials; Prepare fills them in
	Credentials []credentials.Resolved
	// InstructionsMount is the mount spec for this run's instructions snapshot, empty when disabled.
	InstructionsMount string
}

// buildRequest is what every step of Build needs to know about the run.
type buildRequest struct {
	target        Target
	in            Input
	toolConfig    tools.ToolConfig
	rc            *config.AgenticRC
	containerHome string
}

// New returns a Service that talks to Docker through d and to microsandbox through m.
func New(d Docker, m Microsandbox) *Service {
	return &Service{docker: d, microsandbox: m, syncMarketplaces: marketplace.Sync}
}

// Build assembles the sandbox.RunSpec for target, syncing marketplaces; the runtime readies its own host resources in Run.
func (s *Service) Build(target Target, in Input, toolConfig tools.ToolConfig, rc *config.AgenticRC) (sandbox.RunSpec, error) {
	req := buildRequest{
		target:        target,
		in:            in,
		toolConfig:    toolConfig,
		rc:            rc,
		containerHome: s.docker.ResolveContainerHome(target.ImageName),
	}

	marketplaceMounts, marketplaceNames, err := s.syncToolMarketplaces(in.ToolHome, target.ToolName, toolConfig, rc)
	if err != nil {
		return sandbox.RunSpec{}, err
	}

	mounts, err := newMountSet(req, marketplaceMounts)
	if err != nil {
		return sandbox.RunSpec{}, err
	}

	env, err := s.runEnv(req, mounts, marketplaceNames)
	if err != nil {
		return sandbox.RunSpec{}, err
	}

	logDir := proxyLogDir(in.ToolHome, in.ProxyMode.Enabled())
	return newRunSpec(req, mounts, env, logDir), nil
}

// ToolNeedsMarketplaceSync reports whether tool supports marketplace mounting and has at least one marketplace configured.
func (s *Service) ToolNeedsMarketplaceSync(toolConfig tools.ToolConfig, rc *config.AgenticRC, tool string) bool {
	return needsMarketplaceSync(toolConfig, rc, tool)
}

// buildWithInstructions wraps Build with this run's instructions snapshot mounted in; the returned cleanup func must always be deferred, even on error.
func (s *Service) buildWithInstructions(target Target, in Input, toolConfig tools.ToolConfig, rc *config.AgenticRC) (sandbox.RunSpec, func(), error) {
	content, err := s.BuildInstructions(target, in, toolConfig, rc)
	if err != nil {
		return sandbox.RunSpec{}, noCleanup, fmt.Errorf("build instructions for %s: %w", target.ToolName, err)
	}

	snapshot, err := s.PrepareInstructions(in.ToolHome, toolConfig, content)
	if err != nil {
		return sandbox.RunSpec{}, noCleanup, fmt.Errorf("prepare instructions for %s: %w", target.ToolName, err)
	}

	in.InstructionsMount = snapshot.MountSpec

	rs, err := s.Build(target, in, toolConfig, rc)
	if err != nil {
		snapshot.Cleanup()
		return sandbox.RunSpec{}, noCleanup, err
	}

	return rs, snapshot.Cleanup, nil
}

// runEnv resolves the tool's env, refusing managed names, and adds the credential placeholders and marketplace names.
func (s *Service) runEnv(req buildRequest, mounts mountSet, marketplaceNames []string) ([]string, error) {
	in := req.in

	env := resolve.Env(in.Env, req.rc)
	if err := validateEnv(env, in.ProxyMode.Enabled(), in.DindEnabled); err != nil {
		return nil, err
	}

	caps := s.capabilities(in.Runtime)
	placeholders, err := credentialSetup(in, caps, mounts, env)
	if err != nil {
		return nil, err
	}
	env = append(env, placeholders...)

	// Only proxy injection needs the tool to trust the proxy CA
	if len(in.Credentials) > 0 && caps.CredentialsViaProxy {
		if err := s.checkProxyTrust(req.target); err != nil {
			return nil, err
		}
	}

	// Tells entrypoint.sh exactly which names to register instead of globbing.
	if len(marketplaceNames) > 0 {
		env = append(env, "AGENTIC_MARKETPLACES="+strings.Join(marketplaceNames, ","))
	}

	return env, nil
}

// syncToolMarketplaces syncs tool's configured marketplaces and returns each mount spec plus name.
func (s *Service) syncToolMarketplaces(toolHome, tool string, toolConfig tools.ToolConfig, rc *config.AgenticRC) (mounts, names []string, err error) {
	if !needsMarketplaceSync(toolConfig, rc, tool) {
		return nil, nil, nil
	}

	entries := config.MarketplacesFor(rc, tool)
	mpEntries := make([]marketplace.Entry, len(entries))
	for i, e := range entries {
		mpEntries[i] = marketplace.Entry{Name: e.Name, URL: e.URL}
	}

	baseDir := filepath.Join(toolHome, marketplace.MarketplacesDirName)
	results, err := s.syncMarketplaces(mpEntries, func(e marketplace.Entry) string {
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

// needsMarketplaceSync backs ToolNeedsMarketplaceSync for callers without a Service.
func needsMarketplaceSync(toolConfig tools.ToolConfig, rc *config.AgenticRC, tool string) bool {
	if toolConfig.Runtime.MarketplaceMount == nil {
		return false
	}
	return len(config.MarketplacesFor(rc, tool)) > 0
}

// runVolumes lists the tool's volume specs: config and flag mounts, marketplaces, then instructions.
func runVolumes(req buildRequest, marketplaceMounts []string) []string {
	volumes := resolve.Volumes(req.toolConfig.Runtime.Mounts(), req.in.Volumes, req.rc)
	volumes = append(volumes, marketplaceMounts...)
	if req.in.InstructionsMount != "" {
		volumes = append(volumes, req.in.InstructionsMount)
	}

	return volumes
}

// newRunSpec assembles the RunSpec from the resolved run parts, including the proxy and dind sidecar specs.
func newRunSpec(req buildRequest, mounts mountSet, env []string, logDir string) sandbox.RunSpec {
	in, rc, toolConfig := req.in, req.rc, req.toolConfig
	limits := resolve.ResourceLimitsFor(in.Limits, rc)
	dindLimits := resolve.DindResourceLimitsFor(in.DindLimits, rc, limits)

	return sandbox.NewRunSpec(req.target.ImageName).
		WithToolHome(in.ToolHome).
		WithContainerHome(req.containerHome).
		WithUser(platform.UserGroup()).
		WithTTY(platform.IsTerminal()).
		WithVolumes(mounts.volumes...).
		WithSecrets(mounts.secrets...).
		WithEnv(env...).
		WithSkipEntrypoint(req.target.SkipEntrypoint).
		WithEphemeralDirs(toolConfig.Runtime.EphemeralDirs()...).
		WithLimits(limits).
		WithDryRun(in.DryRun).
		WithCredentials(sandboxCredentials(in.Credentials)...).
		WithProxy(sandbox.ProxySpec{
			Mode:   in.ProxyMode,
			Allow:  resolve.ProxyAllowList(toolConfig.Runtime.AllowedHosts, in.DindEnabled, rc),
			LogDir: logDir,
		}).
		WithDind(sandbox.DindSpec{Enabled: in.DindEnabled, Limits: dindLimits}).
		Build()
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

	if err := marketplace.RecordUsage(baseDir, results, cwd); err != nil {
		logging.Warnf("could not record marketplace usage: %v", err)
	}
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
