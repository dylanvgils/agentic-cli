package run

import (
	"fmt"
	"io"
	"path/filepath"
	"slices"
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

// Capabilities reports what msb runs offer, plus agentic-proxy's features; integrations would need its token, so not yet.
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

// Check refuses a missing msb or one older than minMicrosandbox; a dry run makes no msb calls, so it skips the check.
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

// Run runs rs's image in microsandbox, loading it from Docker first unless msb already holds this exact image and checking
// the mounts again after the load; with dind it runs the image's dockerd image, built first when needed, and with
// agenticProxy it goes through agentic-proxy. A dry run only prints.
func (r microsandboxRunner) Run(req Request, rs sandbox.RunSpec) error {
	image := rs.Image
	if rs.Dind.Enabled {
		image = tools.DockerdImage(rs.Image)
		if !rs.DryRun {
			if err := r.ensureDockerdImage(rs.Image, image); err != nil {
				return err
			}
		}
	}

	ref, err := r.microsandboxRef(image, rs.DryRun)
	if err != nil {
		return err
	}

	if !rs.DryRun {
		if err := r.ensureMicrosandboxImage(image, ref); err != nil {
			return err
		}
	}

	if err := recheckMounts(rs); err != nil {
		return err
	}

	rs.Image = ref
	if !r.agenticProxy || !rs.Egress.Mode.Enabled() {
		return r.microsandbox.Run(rs, nil)
	}
	return r.runThroughProxy(req, rs)
}

// runThroughProxy runs rs through a published agentic-proxy, rechecking mounts after its start, then prints its summary.
func (r microsandboxRunner) runThroughProxy(req Request, rs sandbox.RunSpec) error {
	proxyLog, err := proxyLogFile(rs.ToolHome, true)
	if err != nil {
		return err
	}

	if !rs.DryRun {
		if err := prepareProxyLogDir(filepath.Dir(proxyLog), rs.ToolHome); err != nil {
			return err
		}
		if err := r.sidecarImages.Ensure(true, false, req.Flags.Registry); err != nil {
			return err
		}
		if err := recheckMounts(rs); err != nil {
			return err
		}
	}

	published, err := r.docker.StartPublishedProxy(rs, proxyLog)
	if err != nil {
		return err
	}
	defer func() {
		// Stop first, so requests the proxy logs while stopping make the summary
		published.Stop()
		if !rs.DryRun {
			printProxySummary(proxyLog, rs.Egress.Mode)
		}
	}()

	if err := recheckMounts(rs); err != nil {
		return err
	}
	return r.microsandbox.Run(rs, &microsandbox.ProxyRoute{
		Port:  published.Port,
		Token: published.Token,
		CAPEM: published.CAPEM,
		Env:   published.Env,
	})
}

// microsandboxRef returns image tagged with its Docker image ID, which pins a run to this exact image; a dry run names a
// missing image untagged, since a real run builds it first.
func (r microsandboxRunner) microsandboxRef(image string, dryRun bool) (string, error) {
	info, err := r.docker.InspectImage(image)
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", image, err)
	}
	if info == nil && dryRun {
		return image, nil
	}
	if info == nil {
		return "", fmt.Errorf("image %s not found", image)
	}
	return image + ":" + info.ID, nil
}

// ensureDockerdImage builds dockerd, image's dockerd image, when it's missing, built by another agentic version or
// derived from an older build of image.
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

// ensureMicrosandboxImage loads image into msb as ref unless msb already holds ref, then removes image's older refs.
func (r microsandboxRunner) ensureMicrosandboxImage(image, ref string) error {
	refs, err := r.microsandbox.ImageRefs()
	if err != nil {
		return err
	}
	if slices.ContainsFunc(refs, func(have string) bool { return microsandbox.SameRef(have, ref) }) {
		return nil
	}

	if err := r.loadIntoMicrosandbox(image, ref); err != nil {
		return err
	}

	r.removeStaleRefs(refs, image, ref)
	return nil
}

// loadIntoMicrosandbox streams `docker save image` into `msb load`, tagging the result as ref; a failed save is reported over the
// load error it causes.
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

// removeStaleRefs removes refs to image other than keep and latest; a ref msb can't remove (e.g. in use) is skipped.
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

// dockerdRefreshReason says why the dockerd image current needs a rebuild from parent, or "" if it's current.
func dockerdRefreshReason(current, parent *docker.ImageInfo) string {
	if reason := docker.ImageRefreshReason(current, 0); reason != "" {
		return reason
	}
	if current.DockerdParent != parent.FullID {
		return "tool image changed"
	}
	return ""
}
