package run

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/microsandbox"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
)

// checkMicrosandbox refuses proxy credentials, whose placeholders nothing in the VM would swap for the real secrets, and a
// missing msb; a dry run makes no msb calls, so it skips the msb check.
func (s *Service) checkMicrosandbox(req Request) error {
	if len(req.Project.RC.Run.Proxy.Credentials) > 0 {
		return errors.New("credential injection needs the egress proxy, which --runtime microsandbox doesn't support yet; use --runtime docker")
	}

	if req.Flags.DryRun {
		return nil
	}
	return s.microsandbox.Available()
}

// runMicrosandbox runs rs's image in microsandbox, loading it from Docker first unless msb already holds this exact image; a dry run only prints the msb command.
func (s *Service) runMicrosandbox(rs sandbox.RunSpec, toolArgs []string) error {
	if ignored := microsandbox.Ignored(rs); len(ignored) > 0 {
		logging.Warnf("--runtime microsandbox ignores %s", strings.Join(ignored, ", "))
	}

	ref, err := s.microsandboxRef(rs.Image)
	if err != nil {
		return err
	}

	if !rs.DryRun {
		if err := s.ensureMicrosandboxImage(rs.Image, ref); err != nil {
			return err
		}
	}

	rs.Image = ref
	return s.microsandbox.Run(rs, toolArgs)
}

// microsandboxRef returns image tagged with its Docker image ID, which pins a run to this exact image.
func (s *Service) microsandboxRef(image string) (string, error) {
	info, err := s.docker.InspectImage(image)
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", image, err)
	}
	if info == nil {
		return "", fmt.Errorf("image %s not found", image)
	}
	return image + ":" + info.ID, nil
}

// ensureMicrosandboxImage loads image into msb as ref unless msb already holds ref, then removes image's older refs.
func (s *Service) ensureMicrosandboxImage(image, ref string) error {
	refs, err := s.microsandbox.ImageRefs()
	if err != nil {
		return err
	}
	if slices.ContainsFunc(refs, func(r string) bool { return sameRef(r, ref) }) {
		return nil
	}

	if err := s.loadIntoMicrosandbox(image, ref); err != nil {
		return err
	}

	s.removeStaleRefs(refs, image, ref)
	return nil
}

// loadIntoMicrosandbox streams `docker save image` into `msb load`, tagging the result as ref.
func (s *Service) loadIntoMicrosandbox(image, ref string) error {
	logging.Stepf("Loading %s into microsandbox", image)

	pr, pw := io.Pipe()
	go func() { _ = pw.CloseWithError(s.docker.SaveImage(image, pw)) }()
	if err := s.microsandbox.LoadImage(pr, ref); err != nil {
		_ = pr.CloseWithError(err)
		return err
	}
	return nil
}

// removeStaleRefs removes refs to image other than keep and latest; a ref msb can't remove (e.g. in use) is skipped.
func (s *Service) removeStaleRefs(refs []string, image, keep string) {
	repo, _ := splitRef(image)
	for _, ref := range refs {
		refRepo, tag := splitRef(ref)
		if refRepo != repo || tag == "latest" || sameRef(ref, keep) {
			continue
		}
		if err := s.microsandbox.RemoveImage(ref); err != nil {
			logging.Infof("kept old microsandbox image %s: %v", ref, err)
		}
	}
}

// sameRef reports whether a and b name the same repository and tag, ignoring a registry or namespace prefix msb may add.
func sameRef(a, b string) bool {
	aRepo, aTag := splitRef(a)
	bRepo, bTag := splitRef(b)
	return aRepo == bRepo && aTag == bTag
}

// splitRef returns ref's repository name without any registry or namespace prefix, and its tag ("latest" when unset).
func splitRef(ref string) (repo, tag string) {
	name := ref[strings.LastIndex(ref, "/")+1:]
	repo, tag, ok := strings.Cut(name, ":")
	if !ok {
		tag = "latest"
	}
	return repo, tag
}
