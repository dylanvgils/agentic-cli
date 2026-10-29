// Package run builds the sandbox.RunSpec for `agentic run`, syncing marketplaces and assembling resolved volumes, env vars, and resource limits.
package run

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/git"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
	"github.com/dylanvgils/agentic-cli/internal/network/credentials"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolupdate"
)

// Service builds RunSpecs and run-time instructions for `agentic run`.
type Service struct {
	docker       Docker
	microsandbox Microsandbox
	// syncMarketplaces clones or updates marketplaces with git; tests swap it so they need no network
	syncMarketplaces func(entries []marketplace.Entry, dirFor func(marketplace.Entry) string) ([]marketplace.Result, error)
	// upstream looks up the latest tool versions for the update check; tests swap it so they need no network
	upstream toolupdate.Upstream
}

// Target identifies the tool and image a RunSpec is being built for.
type Target struct {
	ToolName       string
	ImageName      string
	SkipEntrypoint bool
	// ToolArgs are the command line's args after the tool name
	ToolArgs []string
}

// Input carries the flag/env-derived values build needs.
type Input struct {
	ToolHome  string
	Volumes   []string
	Env       []string
	Limits    sandbox.ResourceLimits
	DryRun    bool
	Registry  string
	ProxyMode sandbox.ProxyMode
	// ProxyIntegrations are the integrations the run applies, empty unless the proxy runs on a runtime with them
	ProxyIntegrations []string
	DindEnabled       bool
	// Sidecar limit flags; empty falls back to config
	DindLimits sandbox.ResourceLimits
	// Runtime is where the run executes; empty means Docker
	Runtime sandbox.Runtime
	// Credentials are the approved proxy credentials; Prepare fills them in
	Credentials []credentials.Resolved
	// InstructionsMount is the mount spec for this run's instructions snapshot, empty when disabled.
	InstructionsMount string
}

// buildRequest is what every step of build needs to know about the run.
type buildRequest struct {
	target        Target
	in            Input
	toolConfig    tools.ToolConfig
	rc            *config.AgenticRC
	caps          sandbox.Capabilities
	containerHome string
}

// New returns a Service that talks to Docker through d and to microsandbox through m.
func New(d Docker, m Microsandbox) *Service {
	return &Service{
		docker:           d,
		microsandbox:     m,
		syncMarketplaces: marketplace.NewSyncer(git.New()).Sync,
		upstream:         tools.Upstream{},
	}
}

// build assembles the sandbox.RunSpec for target from the approved config, syncing marketplaces; the runtime readies its
// own host resources in Run.
func (s *Service) build(target Target, in Input, toolConfig tools.ToolConfig, approved approvedConfig) (sandbox.RunSpec, error) {
	rc := approved.rc
	req := buildRequest{
		target:        target,
		in:            in,
		toolConfig:    toolConfig,
		rc:            rc,
		caps:          s.capabilities(in.Runtime),
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

	logFile, err := proxyLogFile(in.ToolHome, in.ProxyMode.Enabled())
	if err != nil {
		return sandbox.RunSpec{}, err
	}
	rs := newRunSpec(req, mounts, env, logFile)

	// Only a proxy that decrypts, for credentials or path rules, needs the tool to trust its CA
	spec := specCapabilities(rs)
	if (spec.CredentialInjection || spec.PathRules) && req.caps.CredentialsViaProxy {
		if err := s.checkProxyTrust(target); err != nil {
			return sandbox.RunSpec{}, err
		}
	}
	return rs, nil
}

// ToolNeedsMarketplaceSync reports whether tool supports marketplace mounting and has at least one marketplace configured.
func (s *Service) ToolNeedsMarketplaceSync(toolConfig tools.ToolConfig, rc *config.AgenticRC, tool string) bool {
	return needsMarketplaceSync(toolConfig, rc, tool)
}

// buildWithInstructions wraps build with this run's instructions snapshot mounted in; the returned cleanup func must always be deferred, even on error.
func (s *Service) buildWithInstructions(target Target, in Input, toolConfig tools.ToolConfig, approved approvedConfig) (sandbox.RunSpec, func(), error) {
	content, err := s.BuildInstructions(target, in, toolConfig, approved.rc)
	if err != nil {
		return sandbox.RunSpec{}, noCleanup, fmt.Errorf("build instructions for %s: %w", target.ToolName, err)
	}

	snapshot, err := s.PrepareInstructions(in.ToolHome, toolConfig, content)
	if err != nil {
		return sandbox.RunSpec{}, noCleanup, fmt.Errorf("prepare instructions for %s: %w", target.ToolName, err)
	}

	in.InstructionsMount = snapshot.MountSpec

	rs, err := s.build(target, in, toolConfig, approved)
	if err != nil {
		snapshot.Cleanup()
		return sandbox.RunSpec{}, noCleanup, err
	}

	return rs, snapshot.Cleanup, nil
}

// runEnv resolves the tool's env, refusing managed names, checks the credentials against it and adds the marketplace names.
func (s *Service) runEnv(req buildRequest, mounts mountSet, marketplaceNames []string) ([]string, error) {
	in := req.in

	env := resolve.Env(in.Env, req.rc)
	if err := validateEnv(env, in.ProxyMode.Enabled(), in.DindEnabled, in.ProxyIntegrations); err != nil {
		return nil, err
	}

	if err := credentialSetup(in, req.caps, mounts, env); err != nil {
		return nil, err
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

	baseDir := filepath.Join(toolHome, marketplace.MarketplacesDirName)
	results, err := s.syncMarketplaces(marketplaceEntries(rc, tool), func(e marketplace.Entry) string {
		return filepath.Join(baseDir, marketplace.CloneDirName(e.URL))
	})
	if err != nil {
		return nil, nil, fmt.Errorf("sync marketplaces for %s: %w", tool, err)
	}

	mounts, names = toolMarketplaceMounts(toolConfig, results)
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
func newRunSpec(req buildRequest, mounts mountSet, env []string, proxyLog string) sandbox.RunSpec {
	in, rc, toolConfig := req.in, req.rc, req.toolConfig
	limits := resolve.ResourceLimitsFor(in.Limits, rc)
	dindLimits := resolve.DindResourceLimitsFor(in.DindLimits, rc, limits)

	rules := proxyRules(toolConfig, in, rc, req.caps)

	return sandbox.NewRunSpec(req.target.ImageName).
		WithToolHome(in.ToolHome).
		WithContainerHome(req.containerHome).
		WithUser(platform.HostUser()).
		WithTTY(platform.IsTerminal()).
		WithVolumes(mounts.volumes...).
		WithEnv(env...).
		WithSkipEntrypoint(req.target.SkipEntrypoint).
		WithToolArgs(resolve.ToolArgs(req.target.ToolName, req.target.ToolArgs, req.target.SkipEntrypoint, rc)...).
		WithEphemeralDirs(toolConfig.Runtime.EphemeralDirs()...).
		WithLimits(limits).
		WithDryRun(in.DryRun).
		WithCredentials(sandboxCredentials(in.Credentials)...).
		WithProxy(sandbox.ProxySpec{
			Mode:         in.ProxyMode,
			Rules:        rules,
			LogFile:      proxyLog,
			Integrations: in.ProxyIntegrations,
		}).
		WithDind(sandbox.DindSpec{Enabled: in.DindEnabled, Limits: dindLimits}).
		Build()
}

// marketplaceEntries returns rc's marketplaces for tool as sync entries.
func marketplaceEntries(rc *config.AgenticRC, tool string) []marketplace.Entry {
	configured := config.MarketplacesFor(rc, tool)
	entries := make([]marketplace.Entry, len(configured))
	for i, m := range configured {
		entries[i] = marketplace.Entry{Name: m.Name, URL: m.URL}
	}
	return entries
}

// toolMarketplaceMounts returns the tool's mount spec and name for each synced marketplace, warning about stale clones.
func toolMarketplaceMounts(toolConfig tools.ToolConfig, results []marketplace.Result) (mounts, names []string) {
	mounts = make([]string, len(results))
	names = make([]string, len(results))
	for i, r := range results {
		if r.Stale {
			logging.Warnf("marketplace %q: %v; using existing clone", r.Entry.Name, r.Warning)
		}
		mounts[i] = toolConfig.Runtime.MarketplaceMount(r.Entry.Name, r.Entry.URL)
		names[i] = r.Entry.Name
	}
	return mounts, names
}

// recordMarketplaceUsage records cwd against each result for `marketplace prune`. Best-effort.
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

// validateEnv rejects entries that override an env var agentic manages, including integration vars.
func validateEnv(entries []string, proxyEnabled, dindEnabled bool, integrations []string) error {
	integrationNames := sandbox.ProxyIntegrationEnvNames(integrations)
	for _, entry := range entries {
		key, _, _ := strings.Cut(entry, "=")
		if sandbox.IsReservedEnvName(key, proxyEnabled, dindEnabled) {
			return fmt.Errorf("--env: %q is managed by agentic and cannot be overridden", key)
		}
		if slices.Contains(integrationNames, key) {
			return fmt.Errorf("--env: %q is set by a proxy integration and cannot be overridden", key)
		}
	}

	return nil
}
