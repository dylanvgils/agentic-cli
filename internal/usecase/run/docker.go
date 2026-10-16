package run

import (
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/sidecar"
)

// dockerRunner runs tools in hardened Docker containers.
type dockerRunner struct {
	docker Docker
}

// Capabilities reports what Docker runs offer.
func (r dockerRunner) Capabilities() sandbox.Capabilities {
	return r.docker.Capabilities()
}

// Check accepts every run, as Docker needs nothing beyond the capabilities.
func (r dockerRunner) Check(Request) error {
	return nil
}

// Run readies the Docker-only host resources rs needs, starts its sidecars and runs it, checking the mounts again right
// before the sidecars and the tool start as readying them can take long enough for a path to change; a dry run only
// checks the image and prints.
func (r dockerRunner) Run(req Request, rs sandbox.RunSpec, toolArgs []string) error {
	if rs.Dind.Enabled {
		if err := r.requireDockerLayer(rs.Image, req.Target.ToolName); err != nil {
			return err
		}
	}

	if !rs.DryRun {
		if err := r.prepareDockerHost(rs, req.Flags.Registry); err != nil {
			return err
		}
	}

	// The dind sidecar mounts the workspace too, and building sidecar images can take a while
	if err := recheckMounts(rs); err != nil {
		return err
	}

	sidecars, err := r.docker.StartSidecars(rs)
	if err != nil {
		return err
	}
	defer r.docker.StopSidecars(sidecars)

	if err := recheckMounts(rs); err != nil {
		return err
	}
	return r.docker.RunTool(sidecars, toolArgs)
}

// prepareDockerHost creates rs's named volumes and proxy log dir, builds missing sidecar images and ensures agentic-net when the tool joins it.
func (r dockerRunner) prepareDockerHost(rs sandbox.RunSpec, registry string) error {
	if err := r.docker.EnsureNamedVolumes(rs.Volumes, rs.ToolHome, rs.ContainerHome, tools.BusyboxImageFor(registry)); err != nil {
		return err
	}

	proxy, dind := rs.Proxy.Mode.Enabled(), rs.Dind.Enabled
	if proxy {
		if err := prepareProxyLogDir(rs.Proxy.LogDir, rs.ToolHome); err != nil {
			return err
		}
	}

	if err := sidecar.New(r.docker).Ensure(proxy, dind, registry); err != nil {
		return err
	}

	// In proxy or dind mode the tool container attaches to a per-run network
	// instead of agentic-net; startProxy ensures agentic-net itself for the
	// sidecar's egress connection, so skip the redundant check here.
	if proxy || dind {
		return nil
	}
	return r.docker.EnsureNetwork()
}
