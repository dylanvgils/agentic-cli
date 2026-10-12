// Package volumes lists and removes agentic's named volumes across the Docker and microsandbox runtimes.
package volumes

import "fmt"

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

// New returns a Service over Docker and microsandbox volumes.
func New(d Docker, m Microsandbox) *Service {
	return &Service{docker: d, msb: m}
}

// List returns agentic's Docker volumes, then its msb volumes when msb is installed.
func (s *Service) List() ([]Volume, error) {
	names, err := s.docker.ListVolumeNames()
	if err != nil {
		return nil, err
	}
	volumes := named(names, RuntimeDocker)

	if s.msb.Available() != nil {
		return volumes, nil
	}
	msbNames, err := s.msb.ListVolumeNames()
	if err != nil {
		return nil, err
	}
	return append(volumes, named(msbNames, RuntimeMicrosandbox)...), nil
}

// Remove removes v from its runtime.
func (s *Service) Remove(v Volume) error {
	if v.Runtime == RuntimeMicrosandbox {
		return s.msb.RemoveVolume(v.Name)
	}
	return s.docker.RemoveVolume(v.Name)
}

// RemoveNamed removes name from every runtime that holds it.
func (s *Service) RemoveNamed(name string) error {
	volumes, err := s.List()
	if err != nil {
		return err
	}

	found := false
	for _, v := range volumes {
		if v.Name != name {
			continue
		}
		found = true
		if err := s.Remove(v); err != nil {
			return err
		}
	}

	if !found {
		return fmt.Errorf("'%s' is not an agentic-managed volume", name)
	}
	return nil
}

// named pairs each of names with runtime.
func named(names []string, runtime string) []Volume {
	volumes := make([]Volume, 0, len(names))
	for _, n := range names {
		volumes = append(volumes, Volume{Name: n, Runtime: runtime})
	}
	return volumes
}
