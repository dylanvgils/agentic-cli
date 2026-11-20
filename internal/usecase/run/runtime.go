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
	// Run readies the runtime's host resources for rs, building missing sidecar images from registry, then runs it,
	// rechecking the mounts right before each process that mounts them starts.
	Run(rs sandbox.RunSpec, registry string) error
	// Teardown removes everything run id left on the runtime; best-effort and idempotent.
	Teardown(toolHome string, id sandbox.RunID) error
}

// Apply runs plan on its runner after refusing a spec that needs a feature the runner lacks.
func (s *Service) Apply(plan *Plan) error {
	if err := checkSpec(plan.spec, plan.runner.Capabilities().Features); err != nil {
		return err
	}
	return plan.runner.Run(plan.spec, plan.registry)
}

// End saves plan's instructions snapshot back and removes its temp dir, warning on a failure.
func (s *Service) End(plan *Plan) {
	warnOnEndRunError(endRun(plan.run))
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

// startRun gives a run its ID, its watchdog unless it's a dry run, then its temp dir, before anything it can leave behind.
func (s *Service) startRun(req Request) (sandbox.RunRef, error) {
	id, err := sandbox.NewRunID()
	if err != nil {
		return sandbox.RunRef{}, err
	}

	run := sandbox.RunRef{
		ID:       id,
		Runtime:  req.runtime(),
		Tool:     req.Target.ToolName,
		ToolHome: req.Flags.ToolHome,
	}
	if !req.Flags.DryRun {
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

// checkRuntime picks req's runner, refuses a run it can't do safely and warns about what it drops, then runs its own
// checks.
func (s *Service) checkRuntime(req Request) (runner, error) {
	r, err := s.runner(req.runtime(), req.proxyBackend())
	if err != nil {
		return nil, err
	}

	if err := checkCapabilities(req, r.Capabilities().Features); err != nil {
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
