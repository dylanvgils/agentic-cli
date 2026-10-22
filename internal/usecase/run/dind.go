package run

import (
	"fmt"
	"slices"

	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
)

// requireDockerLayer errors if req's image exists but lacks the docker layer its Docker daemon needs.
func (r dockerRunner) requireDockerLayer(req Request) error {
	image, tool := req.Target.ImageName, req.Target.ToolName
	info, err := r.docker.InspectImage(image)
	if err != nil {
		return err
	}
	// A missing image is reported by requireImage, which runs next
	if info == nil || slices.Contains(docker.RecoverExtras(info.Base), "docker") {
		return nil
	}

	return fmt.Errorf("%s needs the docker CLI in %q; rebuild with \"agentic build %s --base docker\"", dindSource(req), image, tool)
}
