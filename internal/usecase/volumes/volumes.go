// Package volumes lists and removes agentic's named volumes across every runtime.
package volumes

import (
	"fmt"
	"slices"

	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
)

// Service manages agentic's named volumes in every runtime.
type Service struct {
	docker Docker
	others []Runtime // runtimes besides Docker, listed best-effort
}

// Runtime is a runtime besides Docker that holds named volumes.
type Runtime struct {
	Name  sandbox.Runtime
	Store Store
}

// Volume is one named volume and the runtime that holds it.
type Volume struct {
	Name    string
	Runtime sandbox.Runtime
}

// Label names v, marking the volumes that aren't Docker's with their runtime.
func (v Volume) Label() string {
	if v.Runtime == sandbox.RuntimeDocker {
		return v.Name
	}
	return v.Name + " (" + string(v.Runtime) + ")"
}

// New returns a Service over Docker volumes and those of the other installed runtimes.
func New(d Docker, others ...Runtime) *Service {
	return &Service{docker: d, others: others}
}

// List returns agentic's Docker volumes, then each other runtime's; a runtime that can't list them is skipped with a
// warning.
func (s *Service) List() ([]Volume, error) {
	names, err := s.docker.ListVolumeNames()
	if err != nil {
		return nil, err
	}

	volumes := named(names, sandbox.RuntimeDocker)
	for _, r := range s.others {
		names, err := r.Store.ListVolumeNames()
		if err != nil {
			logging.Warnf("skipped %s volumes: %v", r.Name, err)
			continue
		}
		volumes = append(volumes, named(names, r.Name)...)
	}
	return volumes, nil
}

// Remove removes v from its runtime.
func (s *Service) Remove(v Volume) error {
	store, err := s.store(v.Runtime)
	if err != nil {
		return err
	}
	return store.RemoveVolume(v.Name)
}

// RemoveAll removes each of vols from its runtime, reporting progress; it stops at the first error.
func (s *Service) RemoveAll(vols []Volume) error {
	for _, v := range vols {
		if err := s.Remove(v); err != nil {
			return err
		}
		logging.Stepf("deleted: %s", v.Label())
	}
	return nil
}

// RemoveNamed removes name from every runtime that holds it.
func (s *Service) RemoveNamed(name string) error {
	all, err := s.List()
	if err != nil {
		return err
	}

	matches := slices.DeleteFunc(all, func(v Volume) bool { return v.Name != name })
	if len(matches) == 0 {
		return fmt.Errorf("'%s' is not an agentic-managed volume", name)
	}
	for _, v := range matches {
		if err := s.Remove(v); err != nil {
			return err
		}
	}
	return nil
}

// store returns the store of the runtime named runtime.
func (s *Service) store(runtime sandbox.Runtime) (Store, error) {
	if runtime == sandbox.RuntimeDocker {
		return s.docker, nil
	}
	for _, r := range s.others {
		if r.Name == runtime {
			return r.Store, nil
		}
	}
	return nil, fmt.Errorf("unknown volume runtime %q", runtime)
}

// named pairs each of names with runtime, sorted by name so every runtime lists in the same order.
func named(names []string, runtime sandbox.Runtime) []Volume {
	volumes := make([]Volume, 0, len(names))
	for _, n := range slices.Sorted(slices.Values(names)) {
		volumes = append(volumes, Volume{Name: n, Runtime: runtime})
	}
	return volumes
}
