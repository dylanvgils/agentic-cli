package run

import (
	"fmt"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
)

// dockerEngineName is the engine component Docker Engine reports; others, like Podman, name themselves.
const dockerEngineName = "Engine"

// dockerRunner runs tools in hardened Docker containers.
type dockerRunner struct {
	docker        Docker
	sidecarImages SidecarImages
	// nativeBackend is set when the run asks for msb's native proxy backend, which Docker ignores
	nativeBackend bool
}

// Capabilities reports what Docker runs offer.
func (r dockerRunner) Capabilities() sandbox.Capabilities {
	return r.docker.Capabilities()
}

// Check refuses a dind run on an image without the docker layer, ports on a remote daemon and a Docker Engine older than
// minDockerEngine, and warns that other engines can't be checked and that the native proxy backend is ignored; a dry run
// starts nothing, so it skips the daemon checks.
func (r dockerRunner) Check(req Request) error {
	needs := requiredFeatures(req.Flags, req.Project.RC)
	if r.nativeBackend && (needs.Proxy || needs.CredentialInjection) {
		logging.Warnf("%s is ignored: Docker always uses agentic-proxy", proxyBackendSource(req))
	}

	if resolve.DindEnabled(req.Flags.Dind, req.Project.RC) {
		if err := r.requireDockerLayer(req); err != nil {
			return err
		}
	}

	if req.Flags.DryRun {
		return nil
	}

	if portsSet(req.Flags, req.Project.RC) {
		if err := r.docker.RequireLocalContext(portsSource(req) + " would open the ports on that machine"); err != nil {
			return err
		}
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

// Run readies the host, starts the sidecars and runs the tool, rechecking the mounts right before each starts.
func (r dockerRunner) Run(rs sandbox.RunSpec, registry, logDir string) error {
	proxyLog := proxyLogFile(rs, logDir)
	if err := r.prepareDockerHost(rs, registry); err != nil {
		return err
	}

	// The dind sidecar mounts the workspace too, and building sidecar images can take a while
	if err := recheckMounts(rs); err != nil {
		return err
	}

	sidecars, err := r.docker.StartSidecars(rs, proxyLog)
	if err != nil {
		return err
	}
	defer func() {
		// Stop first, so requests the proxy logs while stopping make the summary
		r.docker.StopSidecars(sidecars)
		if rs.Egress.Mode.Enabled() {
			printProxySummary(proxyLog, rs.Egress.Mode)
		}
	}()

	if err := recheckMounts(rs); err != nil {
		return err
	}
	return r.docker.RunTool(sidecars)
}

// DryRun prints the network, sidecar and tool commands Run would run.
func (r dockerRunner) DryRun(rs sandbox.RunSpec, logDir string) error {
	return r.docker.DryRun(rs, proxyLogFile(rs, logDir))
}

// Teardown removes run id's containers, network and host files.
func (r dockerRunner) Teardown(toolHome string, id sandbox.RunID) error {
	return r.docker.Teardown(toolHome, id)
}

// prepareDockerHost creates rs's named volumes and builds missing sidecar images.
func (r dockerRunner) prepareDockerHost(rs sandbox.RunSpec, registry string) error {
	if err := r.docker.EnsureNamedVolumes(rs, tools.BusyboxImageFor(registry)); err != nil {
		return err
	}
	return r.sidecarImages.Ensure(rs.Egress.Mode.Enabled(), rs.Dind.Enabled, registry)
}
