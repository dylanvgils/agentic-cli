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
)

// Request is an `agentic run` call: what to run, where, and the flag-derived input.
type Request struct {
	Target     Target
	ToolConfig tools.ToolConfig
	Cwd        string
	// Layers are loaded once so the credentials the user approves are the ones the run uses
	Layers   []config.RCLayer
	RC       *config.AgenticRC
	TrustDir bool
	// Proxy holds the proxy flags; Prepare resolves them into Input.ProxyMode after the approvals
	Proxy resolve.ProxyInput
	// Input is the flag-derived input; Prepare fills in ProxyMode and Credentials
	Input Input
	// ApplyUpdate installs a tool update the user accepted
	ApplyUpdate toolupdate.Updater
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

	in, err := s.sidecarInput(req)
	if err != nil {
		return docker.RunSpec{}, func() {}, err
	}

	return s.buildWithInstructions(req.Target, in, req.ToolConfig, req.RC)
}

// checkTool makes sure the tool image exists, offers a due tool update and creates the tool's host files.
func (s *Service) checkTool(req Request, prompter Prompter) error {
	target, home := req.Target, req.Input.ToolHome

	if err := s.requireImage(target.ImageName, target.ToolName); err != nil {
		return err
	}

	err := toolupdate.New(s.docker).Check(home, req.RC, target.ToolName, target.ImageName, prompter.OfferToolUpdate, req.ApplyUpdate)
	if err != nil {
		return err
	}

	if err := req.ToolConfig.Runtime.Setup(home); err != nil {
		return fmt.Errorf("setup %s: %w", target.ToolName, err)
	}

	return nil
}

// sidecarInput resolves the proxy mode and approved credentials into req's input and readies the sidecars it enables; a dry run builds no images.
func (s *Service) sidecarInput(req Request) (Input, error) {
	in := req.Input

	proxyMode, err := resolve.ProxyMode(req.Proxy, req.RC)
	if err != nil {
		return Input{}, err
	}
	in.ProxyMode = proxyMode

	// Credentials force the proxy on (see resolve.ProxyMode), so this is a no-op when it is off
	creds, err := resolveCredentials(req.Layers, in.ToolHome)
	if err != nil {
		return Input{}, err
	}
	if len(creds) > 0 && !in.DryRun {
		logging.Infof("injecting credentials for %s", describeInjection(creds))
	}
	in.Credentials = creds

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
	if err := checkTrust(req.Cwd, req.Input.ToolHome, req.TrustDir, prompter); err != nil {
		return err
	}

	return checkCredentials(req.Layers, req.Input.ToolHome, prompter)
}
