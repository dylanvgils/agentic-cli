package run

import (
	"errors"
	"fmt"
	"io"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// microsandboxRunner runs tools in microsandbox VMs, loading their images from Docker.
type microsandboxRunner struct {
	// docker saves the tool image msb loads and runs agentic-proxy
	docker       Docker
	microsandbox Microsandbox
	// sidecarImages builds the proxy image
	sidecarImages SidecarImages
	// agenticProxy enforces the allowlist with agentic-proxy instead of msb's own rules
	agenticProxy bool
}

// dockerImages is the Docker image store msb loads tool images from.
type dockerImages struct {
	docker Docker
}

// Capabilities reports what msb offers with r's proxy backend.
func (r microsandboxRunner) Capabilities() sandbox.Capabilities {
	return r.microsandbox.Capabilities(r.agenticProxy)
}

// Check refuses a missing or too old msb; a dry run skips it.
func (r microsandboxRunner) Check(req Request) error {
	if req.Flags.DryRun {
		return nil
	}
	if err := r.microsandbox.Available(); err != nil {
		return err
	}

	version, err := r.microsandbox.Version()
	if err != nil {
		return fmt.Errorf("read the msb version: %w", err)
	}
	return requireVersion("msb", version, minMicrosandbox)
}

// Run loads rs's image into msb when needed and runs it, rechecking the mounts right before the VM starts.
func (r microsandboxRunner) Run(rs sandbox.RunSpec, registry, logDir string) error {
	image := rs.Image
	if rs.Dind.Enabled {
		image = tools.DockerdImage(rs.Image)
		if err := r.ensureDockerdImage(rs.Image, image); err != nil {
			return err
		}
	}

	ref, err := r.microsandbox.EnsureImage(dockerImages{docker: r.docker}, image)
	if err != nil {
		return err
	}

	rs.Image = ref
	if r.agenticProxy && rs.Egress.Mode.Enabled() {
		return r.runThroughProxy(rs, registry, logDir)
	}

	if err := recheckMounts(rs); err != nil {
		return err
	}
	return r.microsandbox.Run(rs)
}

// DryRun prints the msb command Run would run, plus the proxy command with agentic-proxy; a missing image is named
// untagged, as a real run builds it first.
func (r microsandboxRunner) DryRun(rs sandbox.RunSpec, logDir string) error {
	image := rs.Image
	if rs.Dind.Enabled {
		image = tools.DockerdImage(rs.Image)
	}

	ref, found, err := r.microsandbox.Ref(dockerImages{docker: r.docker}, image)
	if err != nil {
		return err
	}
	if !found {
		ref = image
	}
	rs.Image = ref

	if !r.agenticProxy || !rs.Egress.Mode.Enabled() {
		return r.microsandbox.DryRun(rs)
	}

	published, err := r.docker.DryRunPublishedProxy(rs, proxyLogFile(rs, logDir), sandbox.RuntimeMicrosandbox)
	if err != nil {
		return err
	}
	return r.microsandbox.DryRunThroughProxy(rs, proxyRoute(published))
}

// Teardown stops run id's VM and removes its published proxy.
func (r microsandboxRunner) Teardown(toolHome string, id sandbox.RunID) error {
	return errors.Join(r.microsandbox.Teardown(id), r.docker.Teardown(toolHome, id))
}

// runThroughProxy runs rs behind a published agentic-proxy, then prints its summary.
func (r microsandboxRunner) runThroughProxy(rs sandbox.RunSpec, registry, logDir string) error {
	proxyLog := proxyLogFile(rs, logDir)
	if err := r.sidecarImages.Ensure(true, false, registry); err != nil {
		return err
	}

	published, err := r.docker.StartPublishedProxy(rs, proxyLog, sandbox.RuntimeMicrosandbox)
	if err != nil {
		return err
	}
	defer func() {
		// Stop first, so requests the proxy logs while stopping make the summary
		published.Stop()
		printProxySummary(proxyLog, rs.Egress.Mode)
	}()

	if err := recheckMounts(rs); err != nil {
		return err
	}
	return r.microsandbox.RunThroughProxy(rs, proxyRoute(published))
}

// ensureDockerdImage builds image's dockerd image when it's missing or stale.
func (r microsandboxRunner) ensureDockerdImage(image, dockerd string) error {
	parent, err := r.docker.InspectImage(image)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", image, err)
	}
	if parent == nil {
		return fmt.Errorf("image %s not found", image)
	}

	current, err := r.docker.InspectImage(dockerd)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", dockerd, err)
	}

	reason := dockerdRefreshReason(current, parent)
	if reason == "" {
		return nil
	}

	logging.Infof("building %s (%s)...", dockerd, reason)
	return r.docker.BuildDockerdImage(dockerd, parent)
}

// ImageID returns image's Docker image ID; found is false when image is missing.
func (d dockerImages) ImageID(image string) (id string, found bool, err error) {
	info, err := d.docker.InspectImage(image)
	if err != nil || info == nil {
		return "", false, err
	}
	return info.ID, true, nil
}

func (d dockerImages) SaveImage(image string, w io.Writer) error {
	return d.docker.SaveImage(image, w)
}

// proxyRoute routes msb through published.
func proxyRoute(published docker.PublishedProxy) microsandbox.ProxyRoute {
	return microsandbox.ProxyRoute{
		Port:  published.Port,
		Token: published.Token,
		CAPEM: published.CAPEM,
		Env:   published.Env,
	}
}

// dockerdRefreshReason says why current needs a rebuild from parent, or "" if it's current.
func dockerdRefreshReason(current, parent *docker.ImageInfo) string {
	if reason := docker.ImageRefreshReason(current, 0); reason != "" {
		return reason
	}
	if current.DockerdParent != parent.FullID {
		return "tool image changed"
	}
	return ""
}
