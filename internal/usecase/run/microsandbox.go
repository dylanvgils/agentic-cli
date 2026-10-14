package run

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/microsandbox"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
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

// Check refuses a missing msb; a dry run makes no msb calls, so it skips the check.
func (r microsandboxRunner) Check(req Request) error {
	if req.Flags.DryRun {
		return nil
	}
	return r.microsandbox.Available()
}

// Run runs rs's image in microsandbox, loading it from Docker first unless msb already holds this exact image and checking
// the mounts again after the load; a dry run only prints the msb command.
func (r microsandboxRunner) Run(_ Request, rs sandbox.RunSpec, toolArgs []string) error {
	ref, err := r.microsandboxRef(rs.Image)
	if err != nil {
		return err
	}

	if !rs.DryRun {
		if err := r.ensureMicrosandboxImage(rs.Image, ref); err != nil {
			return err
		}
	}

	if err := recheckMounts(rs); err != nil {
		return err
	}

	rs.Image = ref
	return r.microsandbox.Run(rs, toolArgs)
}

// microsandboxRef returns image tagged with its Docker image ID, which pins a run to this exact image.
func (r microsandboxRunner) microsandboxRef(image string) (string, error) {
	info, err := r.docker.InspectImage(image)
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", image, err)
	}
	if info == nil {
		return "", fmt.Errorf("image %s not found", image)
	}
	return image + ":" + info.ID, nil
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
