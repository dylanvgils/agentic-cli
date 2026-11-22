// Package run builds the sandbox.RunSpec for `agentic run`, syncing marketplaces and assembling resolved volumes, env vars, and resource limits.
package run

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config/precedence"
	"github.com/dylanvgils/agentic-cli/internal/config/rc"
	"github.com/dylanvgils/agentic-cli/internal/image/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/git"
	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/infra/platform"
	"github.com/dylanvgils/agentic-cli/internal/network/credentials"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// Service builds RunSpecs and run-time instructions for `agentic run`.
type Service struct {
	docker       Docker
	microsandbox Microsandbox
	// syncMarketplaces clones or updates marketplaces with git; tests swap it so they need no network
	syncMarketplaces func(entries []marketplace.Entry, dirFor func(marketplace.Entry) string) ([]marketplace.Result, error)
	toolUpdates      ToolUpdates
	sidecarImages    SidecarImages
	watchdog         Watchdog
	commandLog       CommandLog
}

// Target identifies the tool and image a RunSpec is being built for.
type Target struct {
	ToolName       string
	ImageName      string
	SkipEntrypoint bool
	// ToolArgs are the command line's args after the tool name
	ToolArgs []string
}

// runConfig is a run resolved against its approved config: what building the RunSpec and the instructions read.
type runConfig struct {
	Target Target
	Tool   tools.ToolConfig
	RC     *rc.AgenticRC
	Image  *docker.ImageInfo // the tool image, nil when it isn't built

	Runtime runtimeConfig
	Homes   homes
	Egress  egressConfig
	Limits  sandbox.ResourceLimits // the tool's, resolved against the config
	Dind    sandbox.DindSpec       // Limits resolved against the tool's

	Mounts runMounts
	Env    []string // extra env from the flags; precedence.Env merges the config's
	Ports  []sandbox.PortMapping

	Registry string
	DryRun   bool
}

// runtimeConfig is where the run executes and what that runtime offers.
type runtimeConfig struct {
	Name sandbox.Runtime // empty means Docker
	Caps sandbox.Capabilities
}

// homes are the tool home on the host and the home inside the container.
type homes struct {
	Tool      string
	Container string
}

// runMounts are the mounts a run adds to the tool's own.
type runMounts struct {
	Base         []sandbox.Mount // the tool's own, the flags' and the config's
	Instructions string          // the instructions snapshot, empty when disabled
}

// egressConfig is how the run's egress is controlled.
type egressConfig struct {
	Mode         sandbox.EgressMode
	Backend      string // empty means the runtime's default
	Integrations []string
	Credentials  []credentials.Resolved
}

// New returns a Service over Docker d and microsandbox m, offering tool updates through u, building sidecar images
// through si, guarding runs with w and recording their commands to l.
func New(d Docker, m Microsandbox, u ToolUpdates, si SidecarImages, w Watchdog, l CommandLog) *Service {
	return &Service{
		docker:           d,
		microsandbox:     m,
		syncMarketplaces: marketplace.NewSyncer(git.New()).Sync,
		toolUpdates:      u,
		sidecarImages:    si,
		watchdog:         w,
		commandLog:       l,
	}
}

// build assembles the sandbox.RunSpec for c, syncing marketplaces; the runtime readies its own host resources in Run.
func (s *Service) build(c runConfig) (sandbox.RunSpec, error) {
	marketplaceMounts, marketplaceNames, err := s.syncToolMarketplaces(c.Homes.Tool, c.Target.ToolName, c.Tool, c.RC)
	if err != nil {
		return sandbox.RunSpec{}, err
	}

	mounts, err := newMountSet(c, marketplaceMounts)
	if err != nil {
		return sandbox.RunSpec{}, err
	}

	env, err := runEnv(c, mounts, marketplaceNames)
	if err != nil {
		return sandbox.RunSpec{}, err
	}

	rs := newRunSpec(c, mounts, env)

	// Only a proxy that decrypts, for credentials or path rules, needs the tool to trust its CA
	spec := specFeatures(rs)
	if (spec.CredentialInjection || spec.PathRules) && c.Runtime.Caps.CredentialsViaProxy {
		if err := checkProxyTrust(c.Target, c.Image); err != nil {
			return sandbox.RunSpec{}, err
		}
	}
	return rs, nil
}

// ToolNeedsMarketplaceSync reports whether tool supports marketplace mounting and has at least one marketplace configured.
func (s *Service) ToolNeedsMarketplaceSync(toolConfig tools.ToolConfig, agenticRC *rc.AgenticRC, tool string) bool {
	return needsMarketplaceSync(toolConfig, agenticRC, tool)
}

// buildWithInstructions wraps build with run id's instructions snapshot mounted in; endRun saves it back.
func (s *Service) buildWithInstructions(id sandbox.RunID, c runConfig) (sandbox.RunSpec, error) {
	content, err := s.buildInstructions(c)
	if err != nil {
		return sandbox.RunSpec{}, fmt.Errorf("build instructions for %s: %w", c.Target.ToolName, err)
	}

	c.Mounts.Instructions, err = s.prepareInstructions(c, id, content)
	if err != nil {
		return sandbox.RunSpec{}, fmt.Errorf("prepare instructions for %s: %w", c.Target.ToolName, err)
	}

	rs, err := s.build(c)
	if err != nil {
		return sandbox.RunSpec{}, err
	}

	rs.RunID = id
	return rs, nil
}

// syncToolMarketplaces syncs tool's configured marketplaces and returns each mount spec plus name.
func (s *Service) syncToolMarketplaces(toolHome, tool string, toolConfig tools.ToolConfig, agenticRC *rc.AgenticRC) (mounts, names []string, err error) {
	if !needsMarketplaceSync(toolConfig, agenticRC, tool) {
		return nil, nil, nil
	}

	baseDir := filepath.Join(toolHome, marketplace.MarketplacesDirName)
	results, err := s.syncMarketplaces(marketplaceEntries(agenticRC, tool), func(e marketplace.Entry) string {
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
func needsMarketplaceSync(toolConfig tools.ToolConfig, agenticRC *rc.AgenticRC, tool string) bool {
	if toolConfig.Runtime.MarketplaceMount == nil {
		return false
	}
	return len(rc.MarketplacesFor(agenticRC, tool)) > 0
}

// runEnv resolves the tool's env, refusing managed names, checks the credentials against it and adds the marketplace names.
func runEnv(c runConfig, mounts mountSet, marketplaceNames []string) ([]string, error) {
	env := precedence.Env(c.Env, c.RC)
	if err := validateEnv(env, c.Egress.Mode.Enabled(), c.Dind.Enabled, c.Egress.Integrations); err != nil {
		return nil, err
	}

	if err := credentialSetup(c, mounts, env); err != nil {
		return nil, err
	}

	// Tells entrypoint.sh exactly which names to register instead of globbing.
	if len(marketplaceNames) > 0 {
		env = append(env, "AGENTIC_MARKETPLACES="+strings.Join(marketplaceNames, ","))
	}

	return env, nil
}

// runMountList lists the tool's mounts: its own, flag and config mounts, marketplaces, then instructions.
func runMountList(c runConfig, marketplaceMounts []string) ([]sandbox.Mount, error) {
	mounts := slices.Clone(c.Mounts.Base)
	specs := marketplaceMounts
	if c.Mounts.Instructions != "" {
		specs = append(slices.Clone(specs), c.Mounts.Instructions)
	}
	for _, spec := range specs {
		m, err := sandbox.ParseMount(spec, c.Homes.Tool, c.Homes.Container)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, m)
	}
	return mounts, nil
}

// newRunSpec assembles the RunSpec from the resolved run parts, including the egress policy and the Docker daemon.
func newRunSpec(c runConfig, mounts mountSet, env []string) sandbox.RunSpec {
	return sandbox.NewRunSpec(c.Target.ImageName).
		WithToolHome(c.Homes.Tool).
		WithContainerHome(c.Homes.Container).
		WithUser(platform.HostUser()).
		WithTTY(iostreams.IsTerminal()).
		WithMounts(mounts.mounts...).
		WithEnv(env...).
		WithPorts(c.Ports...).
		WithSkipEntrypoint(c.Target.SkipEntrypoint).
		WithToolArgs(precedence.ToolArgs(c.Target.ToolName, c.Target.ToolArgs, c.Target.SkipEntrypoint, c.RC)...).
		WithEphemeralDirs(c.Tool.Runtime.EphemeralDirs()...).
		WithLimits(c.Limits).
		WithCredentials(sandboxCredentials(c.Egress.Credentials)...).
		WithEgress(sandbox.EgressPolicy{
			Mode:         c.Egress.Mode,
			Rules:        proxyRules(c),
			Integrations: c.Egress.Integrations,
		}).
		WithDind(c.Dind).
		Build()
}

// marketplaceEntries returns rc's marketplaces for tool as sync entries.
func marketplaceEntries(agenticRC *rc.AgenticRC, tool string) []marketplace.Entry {
	configured := rc.MarketplacesFor(agenticRC, tool)
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

// validateEnv rejects entries that override an env var agentic manages.
func validateEnv(entries []string, proxyEnabled, dindEnabled bool, integrations []string) error {
	for _, entry := range entries {
		key, _, _ := strings.Cut(entry, "=")
		if isReservedEnvName(key, proxyEnabled, dindEnabled, integrations) {
			return fmt.Errorf("--env: %q is managed by agentic and cannot be overridden", key)
		}
	}

	return nil
}

// isReservedEnvName reports whether key is an env var agentic manages in a run with or without the proxy and dind, or one
// of the integrations sets, so an env entry can't override it.
func isReservedEnvName(key string, proxyEnabled, dindEnabled bool, integrations []string) bool {
	return sandbox.IsReservedEnvName(key, proxyEnabled, dindEnabled) ||
		slices.Contains(docker.ProxyIntegrationEnvNames(integrations), key)
}
