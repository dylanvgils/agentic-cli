// Package toolruns lists the tools running on every runtime.
package toolruns

import (
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
	var runs []Run
	add := func(run Run) {
		// Sidecars have no tool
		if run.Tool != "" && (namespace == "" || run.Namespace == namespace) {
			runs = append(runs, run)
		}
	}

	if err := s.docker.CheckDaemon(); err != nil {
		logging.Warnf("skipping Docker: %v", err)
	} else if containers, err := s.docker.ListRunningContainers(); err != nil {
		logging.Warnf("list Docker containers: %v", err)
	} else {
		for _, c := range containers {
			add(Run{
				Tool:      c.Tool,
				Namespace: c.Namespace,
				Runtime:   sandbox.RuntimeDocker,
				Status:    c.Status,
				Name:      c.Name,
			})
		}
	}

	if s.msb.Available() != nil {
		return runs
	}
	sandboxes, err := s.msb.ListRunning()
	if err != nil {
		logging.Warnf("%v", err)
		return runs
	}
	for _, sb := range sandboxes {
		add(Run{
			Tool:      sb.Tool,
			Namespace: sb.Namespace,
			Runtime:   sandbox.RuntimeMicrosandbox,
			Status:    sb.Status,
			Name:      sb.Name,
		})
	}
	return runs
}
