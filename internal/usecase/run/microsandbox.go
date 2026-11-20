package run

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

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

// Capabilities reports what msb offers, plus agentic-proxy's features when it's the backend.
func (r microsandboxRunner) Capabilities() sandbox.Capabilities {
	caps := r.microsandbox.Capabilities()
	if r.agenticProxy {
		caps.ProxyMonitor = true
		caps.PathRules = true
		caps.CredentialsViaProxy = true
		caps.EgressViaProxy = true
	}
	return caps
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
func (r microsandboxRunner) Run(rs sandbox.RunSpec, registry string) error {
	image := rs.Image
	if rs.Dind.Enabled {
		image = tools.DockerdImage(rs.Image)
		if err := r.ensureDockerdImage(rs.Image, image); err != nil {
			return err
		}
	}

	ref, found, err := r.microsandboxRef(image)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("image %s not found", image)
	}

	if err := r.ensureMicrosandboxImage(image, ref); err != nil {
		return err
	}

	rs.Image = ref
	if r.agenticProxy && rs.Egress.Mode.Enabled() {
		return r.runThroughProxy(rs, registry)
	}

	if err := recheckMounts(rs); err != nil {
		return err
	}
	return r.microsandbox.Run(rs)
}

// DryRun prints the msb command Run would run, plus the proxy command with agentic-proxy; a missing image is named
// untagged, as a real run builds it first.
func (r microsandboxRunner) DryRun(rs sandbox.RunSpec) error {
	image := rs.Image
	if rs.Dind.Enabled {
		image = tools.DockerdImage(rs.Image)
	}

	ref, found, err := r.microsandboxRef(image)
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

	published, err := r.docker.DryRunPublishedProxy(rs, proxyLogFile(rs), sandbox.RuntimeMicrosandbox)
	if err != nil {
		return err
	}
	return r.microsandbox.DryRunThroughProxy(rs, proxyRoute(published))
}

// runThroughProxy runs rs behind a published agentic-proxy, then prints its summary.
func (r microsandboxRunner) runThroughProxy(rs sandbox.RunSpec, registry string) error {
	proxyLog := proxyLogFile(rs)
	if err := prepareProxyLogDir(filepath.Dir(proxyLog), rs.ToolHome); err != nil {
		return err
	}
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

// Teardown stops run id's VM and removes its published proxy.
func (r microsandboxRunner) Teardown(toolHome string, id sandbox.RunID) error {
	return errors.Join(r.microsandbox.Teardown(id), r.docker.Teardown(toolHome, id))
}

// microsandboxRef returns image tagged with its Docker image ID, pinning the run to this build; found is false when
// image is missing.
func (r microsandboxRunner) microsandboxRef(image string) (ref string, found bool, err error) {
	info, err := r.docker.InspectImage(image)
	if err != nil {
		return "", false, fmt.Errorf("inspect %s: %w", image, err)
	}
	if info == nil {
		return "", false, nil
	}
	return image + ":" + info.ID, true, nil
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

// ensureMicrosandboxImage loads image into msb as ref unless msb holds it, then removes older refs.
func (r microsandboxRunner) ensureMicrosandboxImage(image, ref string) error {
	have, err := r.microsandbox.HasImage(ref)
	if err != nil {
		return err
	}
	if have {
		return nil
	}

	if err := r.loadIntoMicrosandbox(image, ref); err != nil {
		return err
	}

	refs, err := r.microsandbox.ImageRefs()
	if err != nil {
		logging.Warnf("kept old microsandbox images of %s: %v", image, err)
		return nil
	}
	r.removeStaleRefs(refs, image, ref)
	return nil
}

// loadIntoMicrosandbox streams `docker save image` into `msb load`, tagged as ref.
func (r microsandboxRunner) loadIntoMicrosandbox(image, ref string) error {
	logging.Infof("loading %s into microsandbox", image)

	// An untagged name would save every tag of the repo
	if !strings.Contains(image[strings.LastIndex(image, "/")+1:], ":") {
		image += ":latest"
	}

	pr, pw := io.Pipe()
	saved := make(chan error, 1)
	go func() {
		err := r.docker.SaveImage(image, pw)
		_ = pw.CloseWithError(err)
		saved <- err
	}()

	loadErr := r.microsandbox.LoadImage(pr, ref)
	_ = pr.CloseWithError(loadErr)
	if saveErr := <-saved; saveErr != nil {
		return saveErr
	}
	return loadErr
}

// removeStaleRefs removes image's refs other than keep and latest, skipping ones in use.
func (r microsandboxRunner) removeStaleRefs(refs []string, image, keep string) {
	repo, _ := microsandbox.SplitRef(image)
	for _, ref := range refs {
		refRepo, tag := microsandbox.SplitRef(ref)
		if refRepo != repo || tag == "latest" || microsandbox.SameRef(ref, keep) {
			continue
		}
		if err := r.microsandbox.RemoveImage(ref); err != nil {
			logging.Warnf("kept old microsandbox image %s: %v", ref, err)
		}
	}
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
