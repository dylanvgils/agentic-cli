// Package toolruns lists the tools running on every runtime.
package toolruns

import (
	"slices"

	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// Service lists the tools running in Docker containers and microsandbox VMs.
type Service struct {
	docker Docker
	msb    Microsandbox
}

// Run is a running tool, in a container or a VM.
type Run struct {
	Tool      string
	Namespace string
	Runtime   sandbox.Runtime
	Image     string
	Status    string
	Name      string
}

// New returns a Service over Docker and microsandbox.
func New(d Docker, msb Microsandbox) *Service {
	return &Service{docker: d, msb: msb}
}

// List returns the tools running in namespace, or in every namespace when it is empty, leaving out sidecars; a runtime
// it can't list gets a warning.
func (s *Service) List(namespace string) []Run {
	runs := append(s.dockerRuns(), s.msbRuns()...)
	return slices.DeleteFunc(runs, func(r Run) bool {
		// Sidecars have no tool
		return r.Tool == "" || (namespace != "" && r.Namespace != namespace)
	})
}

// dockerRuns returns what runs in Docker containers, or nothing with a warning when Docker can't be listed.
func (s *Service) dockerRuns() []Run {
	if err := s.docker.CheckDaemon(); err != nil {
		logging.Warnf("skipping Docker: %v", err)
		return nil
	}
	containers, err := s.docker.ListRunningContainers()
	if err != nil {
		logging.Warnf("list Docker containers: %v", err)
		return nil
	}

	runs := make([]Run, len(containers))
	for i, c := range containers {
		runs[i] = Run{
			Tool:      c.Tool,
			Namespace: c.Namespace,
			Runtime:   sandbox.RuntimeDocker,
			Image:     c.Image,
			Status:    c.Status,
			Name:      c.Name,
		}
	}
	return runs
}

// msbRuns returns what runs in microsandbox VMs, or nothing when msb isn't installed or, with a warning, can't list them.
func (s *Service) msbRuns() []Run {
	if s.msb.Available() != nil {
		return nil
	}
	sandboxes, err := s.msb.ListRunning()
	if err != nil {
		logging.Warnf("%v", err)
		return nil
	}

	runs := make([]Run, len(sandboxes))
	for i, sb := range sandboxes {
		runs[i] = Run{
			Tool:      sb.Tool,
			Namespace: sb.Namespace,
			Runtime:   sandbox.RuntimeMicrosandbox,
			Image:     sb.Image,
			Status:    sb.Status,
			Name:      sb.Name,
		}
	}
	return runs
}
