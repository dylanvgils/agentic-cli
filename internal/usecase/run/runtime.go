package run

import (
	"errors"
	"fmt"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config/agenticrc"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// runner executes prepared runs on one runtime.
type runner interface {
	// Capabilities reports what runs on the runtime offer and how they isolate the tool.
	Capabilities() sandbox.Capabilities
	// Check refuses a run the runtime can't do for its own reasons, before any prompt.
	Check(req Request) error
	// Run readies the runtime's host resources for rs, building missing sidecar images from registry, then runs it,
	// rechecking the mounts right before each process that mounts them starts; logs go in the existing logDir.
	Run(rs sandbox.RunSpec, registry, logDir string) error
	// DryRun prints the commands Run would run with logDir, starting nothing.
	DryRun(rs sandbox.RunSpec, logDir string) error
	// Teardown removes everything run id left on the runtime; best-effort and idempotent.
	Teardown(toolHome string, id sandbox.RunID) error
}

// Apply starts plan's run, runs it on its runner, recording its commands in its log dir, then ends it.
func (s *Service) Apply(plan *Plan) error {
	run, rs, err := s.start(plan, true)
	if err != nil {
		return err
	}
	defer func() { warnOnEndRunError(endRun(run)) }()

	logDir := runLogDir(run, time.Now())
	if err := s.openRunLog(logDir, run.ToolHome); err != nil {
		return err
	}
	defer s.closeRunLog()
	return plan.runner.Run(rs, plan.config.Registry, logDir)
}

// DryRun prints the commands Apply would run; its run gets a temp dir for the snapshot but no watchdog.
func (s *Service) DryRun(plan *Plan) error {
	run, rs, err := s.start(plan, false)
	if err != nil {
		return err
	}
	defer func() { warnOnEndRunError(endRun(run)) }()
	return plan.runner.DryRun(rs, runLogDir(run, time.Now()))
}

// Teardown removes everything run left: its runtime resources first, so nothing writes its host files while endRun
// handles them. Best-effort and idempotent.
func (s *Service) Teardown(run sandbox.RunRef) error {
	r, err := s.runner(run.Runtime, "")
	if err != nil {
		return err
	}

	runtimeErr := r.Teardown(run.ToolHome, run.ID)
	return errors.Join(runtimeErr, endRun(run))
}

// start starts plan's run, watched when watch is set, and builds its RunSpec, refusing one that needs a feature the
// runner lacks; on an error it ends the run.
func (s *Service) start(plan *Plan, watch bool) (sandbox.RunRef, sandbox.RunSpec, error) {
	run, err := s.startRun(plan.config, watch)
	if err != nil {
		return sandbox.RunRef{}, sandbox.RunSpec{}, err
	}

	rs, err := s.buildWithInstructions(run.ID, plan.config)
	if err == nil {
		err = checkSpec(rs, plan.runner.Capabilities().Features)
	}
	if err != nil {
		warnOnEndRunError(endRun(run))
		return sandbox.RunRef{}, sandbox.RunSpec{}, err
	}
	return run, rs, nil
}

// startRun gives c's run its ID, its watchdog when watch is set, then its temp dir, before anything it can leave
// behind.
func (s *Service) startRun(c runConfig, watch bool) (sandbox.RunRef, error) {
	id, err := sandbox.NewRunID()
	if err != nil {
		return sandbox.RunRef{}, err
	}

	run := sandbox.RunRef{
		ID:       id,
		Runtime:  c.Runtime.Name,
		Tool:     c.Target.ToolName,
		ToolHome: c.Homes.Tool,
	}
	if watch {
		if err := s.watchdog.Spawn(run); err != nil {
			logging.Warnf("watchdog not confirmed, so a killed run may be left behind: %v", err)
		}
	}

	if err := sandbox.CreateRunTempDir(id); err != nil {
		return sandbox.RunRef{}, err
	}
	return run, nil
}

// runner returns the runner for runtime with proxy backend; an empty runtime is Docker, an empty backend the runtime's
// default.
func (s *Service) runner(runtime sandbox.Runtime, backend string) (runner, error) {
	switch runtime {
	case sandbox.RuntimeDocker, "":
		return dockerRunner{
			docker:        s.docker,
			sidecarImages: s.sidecarImages,
			nativeBackend: backend == agenticrc.BackendNative,
		}, nil
	case sandbox.RuntimeMicrosandbox:
		return microsandboxRunner{
			docker:        s.docker,
			microsandbox:  s.microsandbox,
			sidecarImages: s.sidecarImages,
			agenticProxy:  backend == agenticrc.BackendAgentic,
		}, nil
	default:
		return nil, fmt.Errorf("unknown runtime %q", runtime)
	}
}

// checkRuntime returns choice's runner, refusing a run of req it can't do safely and warning about what it drops, then
// runs its own checks.
func (s *Service) checkRuntime(req Request, choice runtimeChoice) (runner, error) {
	r, err := s.runner(choice.name, choice.backend)
	if err != nil {
		return nil, err
	}

	if err := checkCapabilities(req, choice.name, r.Capabilities().Features); err != nil {
		return nil, err
	}
	if err := r.Check(req); err != nil {
		return nil, err
	}
	return r, nil
}

// endRun saves run's instructions snapshot back to its tool, then removes its temp dir; idempotent.
func endRun(run sandbox.RunRef) error {
	saveErr := saveSnapshot(run)
	return errors.Join(saveErr, sandbox.RemoveRunTempDir(run.ID))
}

// warnOnEndRunError reports a failed endRun without failing the run.
func warnOnEndRunError(err error) {
	if err != nil {
		logging.Warnf("could not clean up the run's host files: %v", err)
	}
}
