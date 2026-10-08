package run

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/docker"
)

// requireImage errors if the image doesn't exist locally, hinting at --namespace if the tool has images under other namespaces.
func (s *Service) requireImage(image, tool string) error {
	info, err := s.docker.InspectImage(image)
	if err != nil {
		return err
	}
	if info != nil {
		return nil
	}

	images, err := s.docker.ListAllImages(docker.ToolFilter(tool))
	if err != nil {
		return err
	}

	var namespaces []string
	for _, img := range images {
		namespaces = append(namespaces, img.Namespace)
	}

	if len(namespaces) == 0 {
		return fmt.Errorf("image %q not found; run \"agentic build %s\" to build it", image, tool)
	}

	noun := "namespace"
	if len(namespaces) > 1 {
		noun = "namespaces"
	}
	return fmt.Errorf("image %q not found; %q is available under %s %s - use --namespace or run \"agentic build %s\"",
		image, tool, noun, strings.Join(namespaces, ", "), tool)
}
