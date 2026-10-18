package run

import (
	"fmt"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/credentials"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolupdate"
	"github.com/dylanvgils/agentic-cli/internal/usecase/update"
)

// Request is an `agentic run` call: what to run, in which project, with which flags.
type Request struct {
	Target  Target
	Tool    tools.ToolConfig
	Project Project
	Flags   Flags
}

// Project is the working dir a run is for and its config.
type Project struct {
	Dir string
	// Layers are loaded once so the credentials the user approves are the ones the run uses
	Layers []config.RCLayer
	RC     *config.AgenticRC
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
	Secrets    []string
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
	if err := checkTrust(req.Project.Dir, home, req.Flags.TrustDir, prompter); err != nil {
		return sandbox.RunSpec{}, noCleanup, err
	}

	approved, err := checkSettings(req.Project, home, prompter)
	if err != nil {
		return sandbox.RunSpec{}, noCleanup, err
	}

	if err := s.checkTool(req, prompter); err != nil {
		return sandbox.RunSpec{}, noCleanup, err
	}

	in, err := runInput(req.Flags, approved, s.capabilities(req.runtime()))
	if err != nil {
		return sandbox.RunSpec{}, noCleanup, err
	}

	return s.buildWithInstructions(req.Target, in, req.Tool, approved)
}

// checkTool offers a due tool update and creates the tool's host files.
func (s *Service) checkTool(req Request, prompter Prompter) error {
	target, home, rc := req.Target, req.Flags.ToolHome, req.Project.RC

	updateReq := toolupdate.Request{Home: home, RC: rc, Tool: target.ToolName, Image: target.ImageName}
	apply := func(tool, image string) error {
		return update.New(s.docker, s.upstream, home).ApplyRecovered(tool, image, rc)
	}
	if err := toolupdate.New(s.docker, s.upstream).Check(updateReq, prompter.OfferToolUpdate, apply); err != nil {
		return err
	}

	if err := req.Tool.Runtime.Setup(home); err != nil {
		return fmt.Errorf("setup %s: %w", target.ToolName, err)
	}

	return nil
}

// CheckProjectDir refuses a working dir Docker can't bind-mount; run it before reading the dir's config.
func CheckProjectDir(dir string) error {
	if mount.IsUNCPath(dir) {
		return fmt.Errorf("working directory %q is on a network share; Docker cannot bind-mount UNC paths", dir)
	}
	return nil
}

// runInput resolves flags against the approved config, credentials included, into build's input for a runtime offering caps.
func runInput(flags Flags, approved approvedConfig, caps sandbox.Capabilities) (Input, error) {
	in, err := resolveInput(flags, approved.rc, caps)
	if err != nil {
		return Input{}, err
	}

	creds, err := resolveCredentials(approved.layers, in.ToolHome)
	if err != nil {
		return Input{}, err
	}
	if len(creds) > 0 && !in.DryRun {
		logging.Infof("injecting credentials for %s", describeInjection(creds))
	}
	in.Credentials = creds

	return in, nil
}

// resolveInput resolves flags against rc into build's input for a runtime offering caps, minus the credentials only a run reads.
func resolveInput(flags Flags, rc *config.AgenticRC, caps sandbox.Capabilities) (Input, error) {
	proxyMode, err := resolve.ProxyMode(flags.Proxy, rc, caps.CredentialsViaProxy)
	if err != nil {
		return Input{}, err
	}

	// A runtime without a daemon ignores dind, so its env names and registry hosts stay free
	dind := caps.Dind && resolve.DindEnabled(flags.Dind, rc)

	return Input{
		ToolHome:    flags.ToolHome,
		Volumes:     flags.Volumes,
		Secrets:     flags.Secrets,
		Env:         flags.Env,
		Limits:      flags.Limits,
		DryRun:      flags.DryRun,
		Registry:    flags.Registry,
		ProxyMode:   proxyMode,
		DindEnabled: dind,
		DindLimits:  flags.DindLimits,
		Runtime:     resolve.Runtime(flags.Runtime, rc),
	}, nil
}

// noCleanup is the cleanup for paths that set nothing up.
func noCleanup() {
	// Nothing to undo
}
