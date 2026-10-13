package run

import (
	"fmt"
	"slices"

	"github.com/dylanvgils/agentic-cli/internal/docker"
)

// requireDockerLayer errors if image lacks the docker layer that --dind needs.
func (r dockerRunner) requireDockerLayer(image, tool string) error {
	info, err := r.docker.InspectImage(image)
	if err != nil {
		return err
	}
	if info != nil && slices.Contains(docker.RecoverExtras(info.Base), "docker") {
		return nil
	}

	return fmt.Errorf("--dind needs the docker CLI in %q; rebuild with \"agentic build %s --base docker\"", image, tool)
}
