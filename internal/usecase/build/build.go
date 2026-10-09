// Package build builds tool images for `agentic build`, printing generated Dockerfiles instead in dry-run mode.
package build

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolupdate"
)

// Service builds tool images for `agentic build`.
type Service struct {
	docker Docker
	home   string // agentic data dir, for the custom installs store and update check timestamps
}

// New returns a Service that talks to Docker through d and keeps custom installs under home.
func New(d Docker, home string) *Service {
	return &Service{docker: d, home: home}
}

// Apply builds each tool image in names under namespace, announcing the batch and reporting the base/apt overrides in effect for each,
// then prunes the dangling images and build cache the builds left behind. A fresh build has the latest tool, so it resets `agentic run`'s update check timer.
func (s *Service) Apply(names []string, namespace string, opts tools.BuildOptions) error {
	images := make([]string, len(names))
	for i, name := range names {
		image, err := tools.ImageName(name, namespace)
		if err != nil {
			return err
		}
		images[i] = image
	}

	logging.Infof("building %d image(s): %s", len(images), strings.Join(images, ", "))

	if err := config.SaveCustomInstalls(s.home, opts.CustomInstalls); err != nil {
		return fmt.Errorf("saving custom installs: %w", err)
	}

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

		if err := toolupdate.MarkChecked(s.home, name); err != nil {
			logging.Warnf("could not record update check for %s: %v", name, err)
		}
	}

	// Best effort: a failed cleanup never fails the build
	_ = s.docker.PruneDangling()
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
