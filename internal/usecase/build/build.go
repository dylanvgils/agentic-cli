// Package build builds tool images for `agentic build`, printing generated Dockerfiles instead in dry-run mode.
package build

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolupdate"
)

// Service builds tool images for `agentic build`.
type Service struct {
	docker Docker
}

// New returns a Service that talks to Docker through d.
func New(d Docker) *Service {
	return &Service{docker: d}
}

// Apply builds each tool image in names under namespace, announcing the batch and reporting the base/apt overrides in effect for each.
// A fresh build has the latest tool, so it resets `agentic run`'s update check timer in toolHome.
func (s *Service) Apply(toolHome string, names []string, namespace string, opts tools.BuildOptions) error {
	images := make([]string, len(names))
	for i, name := range names {
		image, err := tools.ImageName(name, namespace)
		if err != nil {
			return err
		}
		images[i] = image
	}

	logging.Infof("building %d image(s): %s", len(images), strings.Join(images, ", "))

	for i, name := range names {
		image := images[i]

		logging.Step(image)
		if len(opts.BaseOverride) > 0 {
			logging.Detailf("base: %s", strings.Join(opts.BaseOverride, ", "))
		} else if opts.BaseExact {
			logging.Detail("base: (none, exact)")
		}
		if len(opts.AptPackages) > 0 {
			logging.Detailf("apt: %s", strings.Join(opts.AptPackages, ", "))
		} else if opts.AptExact {
			logging.Detail("apt: (none, exact)")
		}

		if err := s.docker.BuildTool(name, image, opts); err != nil {
			return err
		}

		if err := toolupdate.MarkChecked(toolHome, name); err != nil {
			logging.Warnf("could not record update check for %s: %v", name, err)
		}
	}
	return nil
}

// DryRun prints the generated Dockerfile for each tool in names instead of building it.
func (s *Service) DryRun(names []string, opts tools.BuildOptions) error {
	for _, name := range names {
		logging.Step(name)
		content, err := tools.GenerateDockerfile(name, opts)
		if err != nil {
			return err
		}
		if _, err := fmt.Println(content); err != nil {
			return err
		}
	}
	return nil
}
