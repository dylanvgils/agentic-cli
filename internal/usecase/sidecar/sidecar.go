// Package sidecar builds the proxy and DinD sidecar images and decides when they need a refresh.
package sidecar

import (
	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// Service builds and refreshes the sidecar images.
type Service struct {
	docker Docker
}

// New returns a Service that talks to Docker through d.
func New(d Docker) *Service {
	return &Service{docker: d}
}

// Ensure refreshes the image of each enabled sidecar when needed.
func (s *Service) Ensure(proxy, dind bool, registry string) error {
	if proxy {
		if err := s.ensureProxy(registry); err != nil {
			return err
		}
	}

	if dind {
		return s.ensureDind(registry)
	}

	return nil
}

// BuildProxy builds the proxy image unconditionally.
func (s *Service) BuildProxy(opts tools.BuildOptions) error {
	return s.docker.BuildProxyImage(tools.ProxyImage, buildinfo.Version, buildinfo.DevSourceDir(tools.ProxyModulePath), opts)
}

// ensureProxy builds the proxy image if missing or stamped with a different CLI version, so `--proxy` picks up proxy changes shipped with a CLI update.
func (s *Service) ensureProxy(registry string) error {
	info, err := s.docker.InspectImage(tools.ProxyImage)
	if err != nil {
		return err
	}

	reason := docker.ImageRefreshReason(info, 0)
	if reason == "" {
		return nil
	}

	logging.Infof("building %s (%s)...", tools.ProxyImage, reason)
	return s.BuildProxy(tools.BuildOptions{Registry: registry})
}

// ensureDind builds the DinD image if missing, outdated or stale; a failed refresh only warns so offline runs work.
func (s *Service) ensureDind(registry string) error {
	info, err := s.docker.InspectImage(tools.DindImage)
	if err != nil {
		return err
	}

	reason := docker.ImageRefreshReason(info, tools.DindImageMaxAge)
	if reason == "" {
		return nil
	}

	logging.Infof("building %s (%s)...", tools.DindImage, reason)
	err = s.docker.BuildDindImage(tools.DindImage, tools.BuildOptions{Registry: registry})
	if err != nil && info != nil {
		logging.Warnf("could not refresh %s, using the existing image: %v", tools.DindImage, err)
		return nil
	}
	return err
}
