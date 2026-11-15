package run

import (
	"errors"
	"fmt"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// runner executes prepared runs on one runtime.
type runner interface {
	// Capabilities reports what runs on the runtime offer and how they isolate the tool.
	Capabilities() sandbox.Capabilities
	// Check refuses a run the runtime can't do for its own reasons, before any prompt.
	Check(req Request) error
	// Run readies the runtime's host resources for rs, then runs it.
	Run(req Request, rs sandbox.RunSpec) error
	// Teardown removes everything run id left on the runtime; best-effort and idempotent.
	Teardown(toolHome string, id sandbox.RunID) error
}

// Run executes rs, which Prepare gave its RunID, on req's runtime, which first readies its own host resources.
// Every runtime gets rs's security checks here, even one whose runner skips its own.
func (s *Service) Run(req Request, rs sandbox.RunSpec) error {
	r, err := s.runner(req.runtime(), req.proxyBackend())
	if err != nil {
		return err
	}

	if err := checkSpec(rs, r.Capabilities().Features); err != nil {
		return err
	}
	if err := recheckMounts(rs); err != nil {
		return err
	}
	return r.Run(req, rs)
}

// Teardown removes everything run left: its runtime resources, then its instructions snapshot, saved back to the tool's
// file, and its temp dir. Best-effort and idempotent.
func (s *Service) Teardown(run sandbox.RunRef) error {
	r, err := s.runner(run.Runtime, "")
	if err != nil {
		return err
	}

	// In order: the runtime first, so nothing writes the snapshot while it's saved
	runtimeErr := r.Teardown(run.ToolHome, run.ID)
	snapshotErr := finalizeLeftoverSnapshot(run)
	return errors.Join(runtimeErr, snapshotErr, sandbox.RemoveRunTempDir(run.ID))
}

// startRun gives a run its ID, its watchdog unless it's a dry run, then its temp dir, before anything it can leave behind.
func (s *Service) startRun(req Request) (sandbox.RunID, error) {
	id, err := sandbox.NewRunID()
	if err != nil {
		return "", err
	}

	if !req.Flags.DryRun {
		run := sandbox.RunRef{
			ID:       id,
			Runtime:  req.runtime(),
			Tool:     req.Target.ToolName,
			ToolHome: req.Flags.ToolHome,
		}
		if err := s.watchdog.Spawn(run); err != nil {
			logging.Warnf("watchdog not confirmed, so a killed run may be left behind: %v", err)
		}
	}

	if err := sandbox.CreateRunTempDir(id); err != nil {
		return "", err
	}
	return id, nil
}

// runner returns the runner for runtime with proxy backend; an empty runtime is Docker, an empty backend the runtime's
// default.
func (s *Service) runner(runtime sandbox.Runtime, backend string) (runner, error) {
	switch runtime {
	case sandbox.RuntimeDocker, "":
		return dockerRunner{docker: s.docker, sidecarImages: s.sidecarImages}, nil
	case sandbox.RuntimeMicrosandbox:
		return microsandboxRunner{
			docker:        s.docker,
			microsandbox:  s.microsandbox,
			sidecarImages: s.sidecarImages,
			agenticProxy:  backend == config.BackendAgentic,
		}, nil
	default:
		return nil, fmt.Errorf("unknown runtime %q", runtime)
	}
}

// capabilities returns what runtime offers with proxy backend; an unknown runtime offers nothing.
func (s *Service) capabilities(runtime sandbox.Runtime, backend string) sandbox.Capabilities {
	r, err := s.runner(runtime, backend)
	if err != nil {
		return sandbox.Capabilities{}
	}
	return r.Capabilities()
}

// checkRuntime refuses a run req's runtime can't do safely and warns about what it drops, then runs that runtime's own checks.
func (s *Service) checkRuntime(req Request) error {
	r, err := s.runner(req.runtime(), req.proxyBackend())
	if err != nil {
		return err
	}

	if err := checkCapabilities(req, r.Capabilities().Features); err != nil {
		return err
	}
	return r.Check(req)
}

// removeRunTempDir removes run id's temp dir once the run is done, warning rather than failing.
func removeRunTempDir(id sandbox.RunID) {
	if err := sandbox.RemoveRunTempDir(id); err != nil {
		logging.Warnf("could not remove the run's temp files: %v", err)
	}
}
