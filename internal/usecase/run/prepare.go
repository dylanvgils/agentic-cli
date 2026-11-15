package run

import (
	"fmt"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/network/credentials"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
)

// Request is an `agentic run` call: what to run, in which project, with which flags.
type Request struct {
	Target Target
	Tool   tools.ToolConfig
	// Project is loaded once so the credentials the user approves are the ones the run uses
	Project config.Project
	Flags   Flags
}

// Flags are a run's command-line settings, before Prepare resolves them against the project config.
type Flags struct {
	ToolHome   string
	TrustDir   bool
	DryRun     bool
	Registry   string
	Proxy      resolve.ProxyInput
	Dind       resolve.DindInput
	Volumes    []string
	Env        []string
	Limits     sandbox.ResourceLimits
	DindLimits sandbox.ResourceLimits
	// Runtime is the --runtime flag; empty defers to the config
	Runtime sandbox.Runtime
}

// runtime returns where r runs: --runtime, then the config, then Docker.
func (r Request) runtime() sandbox.Runtime {
	return resolve.Runtime(r.Flags.Runtime, r.Project.RC)
}

// proxyBackend returns r's proxy backend: --proxy-backend, then the config, else "" for the runtime's default.
func (r Request) proxyBackend() string {
	return resolve.ProxyBackend(r.Flags.Proxy, r.Project.RC)
}

// Prepare runs every check and host-side step `agentic run` needs, asking prompter where the user must approve, and returns
// the RunSpec plus a cleanup func that must always be deferred, even on error.
func (s *Service) Prepare(req Request, prompter Prompter) (sandbox.RunSpec, func(), error) {
	home := req.Flags.ToolHome

	// Before any prompt, so a run the runtime can't do doesn't waste the user's approvals
	if err := s.checkRuntime(req); err != nil {
		return sandbox.RunSpec{}, noCleanup, err
	}
	if err := credentials.Check(req.Project.RC.Run.Proxy.Credentials); err != nil {
		return sandbox.RunSpec{}, noCleanup, err
	}

	if err := s.requireImage(req.Target.ImageName, req.Target.ToolName); err != nil {
		return sandbox.RunSpec{}, noCleanup, err
	}

	// Before anything acts on the project config, which the agent can edit
	if err := checkTrust(req.Project.WorkDir, home, req.Flags.TrustDir, prompter); err != nil {
		return sandbox.RunSpec{}, noCleanup, err
	}

	approved, err := checkSettings(req.Project, home, prompter)
	if err != nil {
		return sandbox.RunSpec{}, noCleanup, err
	}

	if err := s.checkTool(req, approved, prompter); err != nil {
		return sandbox.RunSpec{}, noCleanup, err
	}

	c, err := s.approvedRun(req, approved)
	if err != nil {
		return sandbox.RunSpec{}, noCleanup, err
	}

	run, err := s.startRun(req)
	if err != nil {
		return sandbox.RunSpec{}, noCleanup, err
	}

	cleanup := func() { warnOnEndRunError(endRun(run)) }
	rs, err := s.buildWithInstructions(run.ID, c)
	if err != nil {
		cleanup()
		return sandbox.RunSpec{}, noCleanup, err
	}
	return rs, cleanup, nil
}

// checkTool offers a due tool update, unless it's a dry run, and creates the tool's host files.
func (s *Service) checkTool(req Request, approved approvedConfig, prompter Prompter) error {
	if !req.Flags.DryRun {
		if err := s.offerToolUpdate(req, approved, prompter); err != nil {
			return err
		}
	}

	if err := req.Tool.Runtime.Setup(req.Flags.ToolHome); err != nil {
		return fmt.Errorf("setup %s: %w", req.Target.ToolName, err)
	}

	return nil
}

// offerToolUpdate offers a due tool update and applies it when the user accepts; the rebuild reads the approved config,
// as it can fall back to its guarded custom installs.
func (s *Service) offerToolUpdate(req Request, approved approvedConfig, prompter Prompter) error {
	u := ToolUpdate{
		Home:  req.Flags.ToolHome,
		RC:    approved.rc,
		Tool:  req.Target.ToolName,
		Image: req.Target.ImageName,
	}
	return s.toolUpdates.Offer(u, prompter.OfferToolUpdate)
}

// CheckProjectDir refuses a working dir Docker can't bind-mount; run it before reading the dir's config.
func CheckProjectDir(dir string) error {
	if sandbox.IsUNCPath(dir) {
		return fmt.Errorf("working directory %q is on a network share; Docker cannot bind-mount UNC paths", dir)
	}
	return nil
}

// approvedRun resolves req against the approved config, credentials included.
func (s *Service) approvedRun(req Request, approved approvedConfig) (runConfig, error) {
	c, err := s.resolveRun(req, approved.rc)
	if err != nil {
		return runConfig{}, err
	}

	creds, err := resolveCredentials(approved.layers, c.Homes.Tool)
	if err != nil {
		return runConfig{}, err
	}
	if len(creds) > 0 && !c.DryRun {
		logging.Infof("injecting credentials for %s", describeInjection(creds))
	}
	c.Egress.Credentials = creds

	return c, nil
}

// resolveRun resolves req against rc, minus the credentials only a run reads.
func (s *Service) resolveRun(req Request, rc *config.AgenticRC) (runConfig, error) {
	flags := req.Flags
	if err := sandbox.CheckEgressIntegrations(flags.Proxy.Integrations); err != nil {
		return runConfig{}, fmt.Errorf("--proxy-integration: %w", err)
	}

	runtime := resolve.Runtime(flags.Runtime, rc)
	backend := resolve.ProxyBackend(flags.Proxy, rc)
	caps := s.capabilities(runtime, backend)

	proxyMode, err := resolve.ProxyMode(flags.Proxy, rc, caps.CredentialsViaProxy)
	if err != nil {
		return runConfig{}, err
	}

	limits := resolve.ResourceLimitsFor(flags.Limits, rc)
	return runConfig{
		Target: req.Target,
		Tool:   req.Tool,
		RC:     rc,
		Runtime: runtimeConfig{
			Name: runtime,
			Caps: caps,
		},
		Homes: homes{
			Tool:      flags.ToolHome,
			Container: s.docker.ResolveContainerHome(req.Target.ImageName),
		},
		Egress: egressConfig{
			Mode:         proxyMode,
			Backend:      backend,
			Integrations: proxyIntegrations(flags.Proxy, rc, proxyMode, caps.Features),
		},
		Limits: limits,
		Dind: sandbox.DindSpec{
			// A runtime without a daemon ignores dind, so its env names and registry hosts stay free
			Enabled: caps.Dind && resolve.DindEnabled(flags.Dind, rc),
			Limits:  resolve.DindResourceLimitsFor(flags.DindLimits, rc, limits),
		},
		Mounts:   runMounts{Extra: flags.Volumes},
		Env:      flags.Env,
		Registry: flags.Registry,
		DryRun:   flags.DryRun,
	}, nil
}

// noCleanup is the cleanup for paths that set nothing up.
func noCleanup() {
	// Nothing to undo
}
