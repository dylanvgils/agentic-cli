package run

import (
	"fmt"

	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// runner executes prepared runs on one runtime.
type runner interface {
	// Capabilities reports what runs on the runtime offer.
	Capabilities() sandbox.Capabilities
	// Check refuses a run the runtime can't do for its own reasons, before any prompt.
	Check(req Request) error
	// Run readies the runtime's host resources for rs, then runs it.
	Run(req Request, rs sandbox.RunSpec) error
}

// Run executes rs on req's runtime, which first readies its own host resources.
// Every runtime gets rs's security checks here, even one whose runner skips its own.
func (s *Service) Run(req Request, rs sandbox.RunSpec) error {
	r, err := s.runner(req.runtime())
	if err != nil {
		return err
	}

	if err := checkSpec(rs, r.Capabilities()); err != nil {
		return err
	}
	if err := recheckMounts(rs); err != nil {
		return err
	}
	return r.Run(req, rs)
}

// runner returns the runner for runtime; an empty runtime is Docker.
func (s *Service) runner(runtime sandbox.Runtime) (runner, error) {
	switch runtime {
	case sandbox.RuntimeDocker, "":
		return dockerRunner{docker: s.docker}, nil
	case sandbox.RuntimeMicrosandbox:
		return microsandboxRunner{docker: s.docker, microsandbox: s.microsandbox}, nil
	default:
		return nil, fmt.Errorf("unknown runtime %q", runtime)
	}
}

// capabilities returns what runtime offers; an unknown runtime offers nothing.
func (s *Service) capabilities(runtime sandbox.Runtime) sandbox.Capabilities {
	r, err := s.runner(runtime)
	if err != nil {
		return sandbox.Capabilities{}
	}
	return r.Capabilities()
}

// checkRuntime refuses a run req's runtime can't do safely and warns about what it drops, then runs that runtime's own checks.
func (s *Service) checkRuntime(req Request) error {
	r, err := s.runner(req.runtime())
	if err != nil {
		return err
	}

	if err := checkCapabilities(req, r.Capabilities()); err != nil {
		return err
	}
	return r.Check(req)
}
