package run

import (
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/sidecar"
)

// runDocker readies the Docker-only host resources rs needs, then runs it; a dry run only checks the image and prints.
func (s *Service) runDocker(req Request, rs docker.RunSpec, toolArgs []string) error {
	if rs.Dind.Enabled {
		if err := s.requireDockerLayer(rs.Image, req.Target.ToolName); err != nil {
			return err
		}
	}

	if !rs.DryRun {
		if err := s.prepareDockerHost(rs, req.Flags.Registry); err != nil {
			return err
		}
	}

	return s.docker.RunContainer(rs, toolArgs)
}

// prepareDockerHost creates rs's named volumes and proxy log dir, builds missing sidecar images and ensures agentic-net when the tool joins it.
func (s *Service) prepareDockerHost(rs docker.RunSpec, registry string) error {
	if err := s.docker.EnsureNamedVolumes(rs.Volumes, rs.ToolHome, rs.ContainerHome, tools.BusyboxImageFor(registry)); err != nil {
		return err
	}

	proxy, dind := rs.Proxy.Mode.Enabled(), rs.Dind.Enabled
	if proxy {
		if err := prepareProxyLogDir(rs.Proxy.LogDir, rs.ToolHome); err != nil {
			return err
		}
	}

	if err := sidecar.New(s.docker).Ensure(proxy, dind, registry); err != nil {
		return err
	}

	// In proxy or dind mode the tool container attaches to a per-run network
	// instead of agentic-net; startProxy ensures agentic-net itself for the
	// sidecar's egress connection, so skip the redundant check here.
	if proxy || dind {
		return nil
	}
	return s.docker.EnsureNetwork()
}
