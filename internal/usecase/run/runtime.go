package run

import (
	"fmt"

	"github.com/dylanvgils/agentic-cli/internal/config"
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
}

// Run executes rs on req's runtime, which first readies its own host resources.
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
