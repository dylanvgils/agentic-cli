package run

import (
	"fmt"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
	"github.com/dylanvgils/agentic-cli/internal/usecase/sidecar"
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
	ToolHome       string
	TrustDir       bool
	DryRun         bool
	Registry       string
	Proxy          resolve.ProxyInput
	Dind           resolve.DindInput
	Volumes        []string
	Secrets        []string
	ReadOnlyMounts []string
	Env            []string
	Limits         docker.ResourceLimits
	DindLimits     docker.ResourceLimits
}

// Prepare runs every check and host-side step `agentic run` needs, asking prompter where the user must approve, and returns
// the RunSpec plus a cleanup func that must always be deferred, even on error.
func (s *Service) Prepare(req Request, prompter Prompter) (docker.RunSpec, func(), error) {
	if err := s.checkTool(req, prompter); err != nil {
		return docker.RunSpec{}, func() {}, err
	}

	if err := checkApprovals(req, prompter); err != nil {
		return docker.RunSpec{}, func() {}, err
	}

	in, err := s.runInput(req)
	if err != nil {
		return docker.RunSpec{}, func() {}, err
	}

	return s.buildWithInstructions(req.Target, in, req.Tool, req.Project.RC)
}

// checkTool makes sure the tool image exists, offers a due tool update and creates the tool's host files.
func (s *Service) checkTool(req Request, prompter Prompter) error {
	target, home, rc := req.Target, req.Flags.ToolHome, req.Project.RC

	if err := s.requireImage(target.ImageName, target.ToolName); err != nil {
		return err
	}

	apply := func(tool, image string) error {
		return update.New(s.docker).ApplyRecovered(tool, image, rc)
	}
	if err := toolupdate.New(s.docker).Check(home, rc, target.ToolName, target.ImageName, prompter.OfferToolUpdate, apply); err != nil {
		return err
	}

	if err := req.Tool.Runtime.Setup(home); err != nil {
		return fmt.Errorf("setup %s: %w", target.ToolName, err)
	}

	return nil
}

// runInput resolves req's flags against the project config into Build's input and readies the sidecars it enables; a dry run builds no images.
func (s *Service) runInput(req Request) (Input, error) {
	flags, rc := req.Flags, req.Project.RC

	proxyMode, err := resolve.ProxyMode(flags.Proxy, rc)
	if err != nil {
		return Input{}, err
	}

	// Credentials force the proxy on (see resolve.ProxyMode), so this is a no-op when it is off
	creds, err := resolveCredentials(req.Project.Layers, flags.ToolHome)
	if err != nil {
		return Input{}, err
	}
	if len(creds) > 0 && !flags.DryRun {
		logging.Infof("injecting credentials for %s", describeInjection(creds))
	}

	in := Input{
		ToolHome:       flags.ToolHome,
		Volumes:        flags.Volumes,
		Secrets:        flags.Secrets,
		ReadOnlyMounts: flags.ReadOnlyMounts,
		Env:            flags.Env,
		Limits:         flags.Limits,
		DryRun:         flags.DryRun,
		Registry:       flags.Registry,
		ProxyMode:      proxyMode,
		DindEnabled:    resolve.DindEnabled(flags.Dind, rc),
		DindLimits:     flags.DindLimits,
		Credentials:    creds,
	}

	if in.DindEnabled {
		if err := s.requireDockerLayer(req.Target.ImageName, req.Target.ToolName); err != nil {
			return Input{}, err
		}
	}

	if !in.DryRun {
		if err := sidecar.New(s.docker).Ensure(in.ProxyMode.Enabled(), in.DindEnabled, in.Registry); err != nil {
			return Input{}, err
		}
	}

	return in, nil
}

// checkApprovals has the user trust the working dir and approve new or changed proxy credentials.
func checkApprovals(req Request, prompter Prompter) error {
	home := req.Flags.ToolHome

	if err := checkTrust(req.Project.Dir, home, req.Flags.TrustDir, prompter); err != nil {
		return err
	}

	return checkCredentials(req.Project.Layers, home, prompter)
}
