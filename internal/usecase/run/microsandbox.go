package run

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/microsandbox"
)

// runMicrosandbox runs rs's image in microsandbox, loading it from Docker first unless msb already holds this exact image; a dry run only prints the msb command.
func (s *Service) runMicrosandbox(rs docker.RunSpec, toolArgs []string) error {
	if ignored := microsandbox.Ignored(rs); len(ignored) > 0 {
		logging.Warnf("--runtime microsandbox ignores %s", strings.Join(ignored, ", "))
	}

	info, err := s.docker.InspectImage(rs.Image)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", rs.Image, err)
	}
	if info == nil {
		return fmt.Errorf("image %s not found", rs.Image)
	}

	// The ID tag pins the run to this exact Docker image
	image := rs.Image
	rs.Image = image + ":" + info.ID

	if !rs.DryRun {
		if err := s.loadIntoMicrosandbox(image, rs.Image); err != nil {
			return err
		}
	}

	return s.microsandbox.Run(rs, toolArgs)
}

// loadIntoMicrosandbox loads image into msb as ref unless msb already holds ref, then removes image's older refs.
func (s *Service) loadIntoMicrosandbox(image, ref string) error {
	refs, err := s.microsandbox.ImageRefs()
	if err != nil {
		return err
	}
	if slices.ContainsFunc(refs, func(r string) bool { return sameRef(r, ref) }) {
		return nil
	}

	logging.Stepf("Loading %s into microsandbox", image)

	pr, pw := io.Pipe()
	go func() { _ = pw.CloseWithError(s.docker.SaveImage(image, pw)) }()
	if err := s.microsandbox.LoadImage(pr, ref); err != nil {
		_ = pr.CloseWithError(err)
		return err
	}

	s.removeStaleRefs(refs, image, ref)
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
