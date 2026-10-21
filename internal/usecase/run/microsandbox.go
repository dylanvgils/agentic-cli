package run

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/microsandbox"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
)

// microsandboxRunner runs tools in microsandbox VMs, loading their images from Docker.
type microsandboxRunner struct {
	// docker saves the tool image msb loads
	docker       Docker
	microsandbox Microsandbox
}

// Capabilities reports what msb runs offer.
func (r microsandboxRunner) Capabilities() sandbox.Capabilities {
	return r.microsandbox.Capabilities()
}

// Check refuses a missing msb or one older than minMicrosandbox and warns that msb ignores the Docker daemon limits; a dry
// run makes no msb calls, so it skips the msb check.
func (r microsandboxRunner) Check(req Request) error {
	if resolve.DindEnabled(req.Flags.Dind, req.Project.RC) && dindLimitsSet(req) {
		logging.Warnf("%s ignores the Docker daemon limits ([run.dind] or --dind-*); dockerd shares the VM's CPUs and memory", runtimeSource(req))
	}

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
// the mounts again after the load; with dind it runs the image's dockerd image, built first when needed. A dry run only
// prints the msb command.
func (r microsandboxRunner) Run(_ Request, rs sandbox.RunSpec) error {
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
	return r.microsandbox.Run(rs)
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
	logging.Stepf("Loading %s into microsandbox", image)

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
			logging.Infof("kept old microsandbox image %s: %v", ref, err)
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

// dindLimitsSet reports whether req sets Docker daemon limits, by flag or in config.
func dindLimitsSet(req Request) bool {
	return req.Flags.DindLimits != (sandbox.ResourceLimits{}) || req.Project.RC.Run.Dind.RCLimits != (config.RCLimits{})
}
