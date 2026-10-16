package run

import (
	"fmt"

	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/sidecar"
)

// dockerEngineName is the engine component Docker Engine reports; others, like Podman, name themselves.
const dockerEngineName = "Engine"

// dockerRunner runs tools in hardened Docker containers.
type dockerRunner struct {
	docker Docker
}

// Capabilities reports what Docker runs offer.
func (r dockerRunner) Capabilities() sandbox.Capabilities {
	return r.docker.Capabilities()
}

// Check refuses a Docker Engine older than minDockerEngine and warns that other engines can't be checked; a dry run starts
// nothing, so it skips the check.
func (r dockerRunner) Check(req Request) error {
	if req.Flags.DryRun {
		return nil
	}

	engine, version, err := r.docker.ServerVersion()
	if err != nil {
		return fmt.Errorf("read the Docker Engine version: %w", err)
	}
	if engine != dockerEngineName {
		logging.Warnf("%s %s isn't Docker Engine, so agentic can't check that it's recent enough to keep --proxy runs from leaking DNS", engine, version)
		return nil
	}
	return requireVersion("Docker Engine", version, minDockerEngine)
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

// prepareDockerHost creates rs's named volumes and proxy log dir and builds missing sidecar images.
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

	return sidecar.New(r.docker).Ensure(proxy, dind, registry)
}
