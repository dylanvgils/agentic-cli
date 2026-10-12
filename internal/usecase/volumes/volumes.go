// Package volumes lists and removes agentic's named volumes across the Docker and microsandbox runtimes.
package volumes

import (
	"fmt"
	"slices"

	"github.com/dylanvgils/agentic-cli/internal/logging"
)

// Runtimes a Volume can live in.
const (
	RuntimeDocker       = "docker"
	RuntimeMicrosandbox = "microsandbox"
)

// Service manages agentic's named volumes in every runtime.
type Service struct {
	docker Docker
	msb    Microsandbox
}

// Volume is one named volume and the runtime that holds it.
type Volume struct {
	Name    string
	Runtime string
}

// Label names v, marking the volumes that aren't Docker's with their runtime.
func (v Volume) Label() string {
	if v.Runtime == RuntimeDocker {
		return v.Name
	}
	return v.Name + " (" + v.Runtime + ")"
}

// New returns a Service over Docker and microsandbox volumes.
func New(d Docker, m Microsandbox) *Service {
	return &Service{docker: d, msb: m}
}

// List returns agentic's Docker volumes, then its msb volumes.
func (s *Service) List() ([]Volume, error) {
	names, err := s.docker.ListVolumeNames()
	if err != nil {
		return nil, err
	}

	msbVolumes, err := s.msbVolumes()
	if err != nil {
		return nil, err
	}
	return append(named(names, RuntimeDocker), msbVolumes...), nil
}

// Remove removes v from its runtime.
func (s *Service) Remove(v Volume) error {
	if v.Runtime == RuntimeMicrosandbox {
		return s.msb.RemoveVolume(v.Name)
	}
	return s.docker.RemoveVolume(v.Name)
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

// msbVolumes returns agentic's msb volumes, or none when msb isn't installed.
func (s *Service) msbVolumes() ([]Volume, error) {
	if s.msb.Available() != nil {
		return nil, nil
	}

	names, err := s.msb.ListVolumeNames()
	if err != nil {
		return nil, err
	}
	return named(names, RuntimeMicrosandbox), nil
}

// named pairs each of names with runtime, sorted by name so every runtime lists in the same order.
func named(names []string, runtime string) []Volume {
	volumes := make([]Volume, 0, len(names))
	for _, n := range slices.Sorted(slices.Values(names)) {
		volumes = append(volumes, Volume{Name: n, Runtime: runtime})
	}
	return volumes
}
