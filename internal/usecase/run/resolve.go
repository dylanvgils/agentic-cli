package run

import (
	"errors"
	"fmt"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/network/credentials"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/precedence"
)

// Request is an `agentic run` call: what to run, in which project, with which flags.
type Request struct {
	Target Target
	Tool   tools.ToolConfig
	// Project is loaded once so the credentials the user approves are the ones the run uses
	Project config.Project
	Flags   Flags
}

// Flags are a run's command-line settings, before Resolve resolves them against the project config.
type Flags struct {
	ToolHome   string
	TrustDir   bool
	DryRun     bool
	Registry   string
	Proxy      precedence.ProxyInput
	Dind       precedence.DindInput
	Volumes    []string
	Env        []string
	Ports      []string
	Limits     sandbox.ResourceLimits
	DindLimits sandbox.ResourceLimits
	// Runtime is the --runtime flag; empty defers to the config
	Runtime sandbox.Runtime
}

// Plan is a run Resolve checked and approved; Apply or DryRun starts, runs and ends it.
type Plan struct {
	runner runner
	config runConfig
}

// runtimeChoice is where a run runs and with which proxy backend.
type runtimeChoice struct {
	name sandbox.Runtime
	// backend is "" for the runtime's default
	backend string
}

// Resolve checks and approves the run req asks for, starting nothing.
func (s *Service) Resolve(req Request, prompter Prompter) (*Plan, error) {
	home := req.Flags.ToolHome

	// Before any prompt, so a run the runtime can't do doesn't waste the user's approvals
	choice := chooseRuntime(req.Flags, req.Project.RC)
	r, err := s.checkRuntime(req, choice)
	if err != nil {
		return nil, err
	}
	if err := checkPorts(req); err != nil {
		return nil, err
	}
	if err := credentials.Check(req.Project.RC.Run.Proxy.Credentials); err != nil {
		return nil, err
	}

	if err := s.requireImage(req.Target.ImageName, req.Target.ToolName); err != nil {
		return nil, err
	}

	// Before anything acts on the project config, which the agent can edit
	if err := checkTrust(req.Project.WorkDir, home, req.Flags.TrustDir, prompter); err != nil {
		return nil, err
	}

	approved, err := checkSettings(req.Project, home, prompter)
	if err != nil {
		return nil, err
	}
	if err := requireSameRuntime(choice, req.Flags, approved.rc); err != nil {
		return nil, err
	}

	if err := s.checkTool(req, approved, prompter); err != nil {
		return nil, err
	}

	c, err := s.approvedRun(req, approved, choice, r)
	if err != nil {
		return nil, err
	}
	return &Plan{
		runner: r,
		config: c,
	}, nil
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

// approvedRun resolves req against the approved config on choice's runner r, credentials included.
func (s *Service) approvedRun(req Request, approved approvedConfig, choice runtimeChoice, r runner) (runConfig, error) {
	c, err := s.resolveRun(req, approved.rc, choice, r)
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

// resolveRun resolves req against rc on choice's runner r, minus the credentials only a run reads.
func (s *Service) resolveRun(req Request, rc *config.AgenticRC, choice runtimeChoice, r runner) (runConfig, error) {
	flags := req.Flags
	if err := sandbox.CheckEgressIntegrations(flags.Proxy.Integrations); err != nil {
		return runConfig{}, fmt.Errorf("--proxy-integration: %w", err)
	}

	caps := r.Capabilities()

	proxyMode, err := precedence.ProxyMode(flags.Proxy, rc, caps.CredentialsViaProxy)
	if err != nil {
		return runConfig{}, err
	}

	ports, err := precedence.Ports(flags.Ports, rc)
	if err != nil {
		return runConfig{}, err
	}

	// After the tool update, which can rebuild the image
	image, err := s.docker.InspectImage(req.Target.ImageName)
	if err != nil {
		return runConfig{}, fmt.Errorf("inspect %s: %w", req.Target.ImageName, err)
	}
	containerHome := "/root"
	if image != nil {
		containerHome = image.ContainerHome
	}

	mounts, err := precedence.Mounts(req.Tool.Runtime.Mounts(), flags.Volumes, rc, flags.ToolHome, containerHome)
	if err != nil {
		return runConfig{}, err
	}

	limits := precedence.ResourceLimitsFor(flags.Limits, rc)
	return runConfig{
		Target: req.Target,
		Tool:   req.Tool,
		RC:     rc,
		Image:  image,
		Runtime: runtimeConfig{
			Name: choice.name,
			Caps: caps,
		},
		Homes: homes{
			Tool:      flags.ToolHome,
			Container: containerHome,
		},
		Egress: egressConfig{
			Mode:         proxyMode,
			Backend:      choice.backend,
			Integrations: proxyIntegrations(flags.Proxy, rc, proxyMode, caps.Features),
		},
		Limits: limits,
		Dind: sandbox.DindSpec{
			// A runtime without a daemon ignores dind, so its env names and registry hosts stay free
			Enabled: caps.Dind && precedence.DindEnabled(flags.Dind, rc),
			Limits:  precedence.DindResourceLimitsFor(flags.DindLimits, rc, limits),
		},
		Mounts:   runMounts{Base: mounts},
		Env:      flags.Env,
		Ports:    ports,
		Registry: flags.Registry,
		DryRun:   flags.DryRun,
	}, nil
}

// chooseRuntime picks a run's runtime and proxy backend: the flags, then rc, then Docker and its default backend.
func chooseRuntime(flags Flags, rc *config.AgenticRC) runtimeChoice {
	return runtimeChoice{
		name:    precedence.Runtime(flags.Runtime, rc),
		backend: precedence.ProxyBackend(flags.Proxy, rc),
	}
}

// requireSameRuntime refuses an approved config that picks another runtime or backend than the checked one, so the run
// never skips the checks that ran before the prompts.
func requireSameRuntime(checked runtimeChoice, flags Flags, approved *config.AgenticRC) error {
	if chooseRuntime(flags, approved) != checked {
		return errors.New("the approved config picks a different runtime than the one checked")
	}
	return nil
}
