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

// Check refuses proxy credentials without env, which nothing in the VM could carry msb's placeholder in, and a missing
// msb; a dry run makes no msb calls, so it skips the msb check.
func (r microsandboxRunner) Check(req Request) error {
	for _, entry := range req.Project.RC.Run.Proxy.Credentials {
		if entry.Preset == "" && len(entry.Env) == 0 {
			return fmt.Errorf("credential for %s needs env with --runtime microsandbox, as the tool must send msb's placeholder", strings.Join(entry.Hosts, ", "))
		}
		if entry.Format != "" {
			logging.Warnf("--runtime microsandbox ignores the credential format for %s; the tool sends the header itself", strings.Join(entry.Hosts, ", "))
		}
	}

	if req.Flags.DryRun {
		return nil
	}
	return r.microsandbox.Available()
}

// Run runs rs's image in microsandbox, loading it from Docker first unless msb already holds this exact image; a dry run only prints the msb command.
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

// loadIntoMicrosandbox streams `docker save image` into `msb load`, tagging the result as ref.
func (r microsandboxRunner) loadIntoMicrosandbox(image, ref string) error {
	logging.Stepf("Loading %s into microsandbox", image)

	pr, pw := io.Pipe()
	go func() { _ = pw.CloseWithError(r.docker.SaveImage(image, pw)) }()
	if err := r.microsandbox.LoadImage(pr, ref); err != nil {
		_ = pr.CloseWithError(err)
		return err
	}
	return nil
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
