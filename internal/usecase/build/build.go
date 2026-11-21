// Package build builds tool images for `agentic build`, printing generated Dockerfiles instead in dry-run mode.
package build

import (
	"fmt"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// Service builds tool images for `agentic build`.
type Service struct {
	docker Docker
	home   string // agentic data dir, for the custom installs store and update check timestamps
}

// Target is one tool image to build.
type Target struct {
	Name  string
	Image string
}

// Plan is what Apply builds, with options shared by every target.
type Plan struct {
	Targets []Target
	Opts    tools.BuildOptions
}

// New returns a Service that talks to Docker through d and keeps custom installs under home.
func New(d Docker, home string) *Service {
	return &Service{docker: d, home: home}
}

// Resolve returns the plan to build names in namespace.
func (s *Service) Resolve(names []string, namespace string, opts tools.BuildOptions) (Plan, error) {
	targets := make([]Target, len(names))
	for i, name := range names {
		image, err := tools.ImageName(name, namespace)
		if err != nil {
			return Plan{}, err
		}
		targets[i] = Target{Name: name, Image: image}
	}
	return Plan{Targets: targets, Opts: opts}, nil
}

// DryRun prints each target's Dockerfile instead of building it; labels go to stderr so stdout stays buildable.
func (s *Service) DryRun(plan Plan) error {
	for _, t := range plan.Targets {
		content, err := tools.GenerateDockerfile(t.Name, plan.Opts)
		if err != nil {
			return err
		}
		sandbox.PrintDryRunText(t.Name+" Dockerfile", content)
	}
	return nil
}

// Apply builds each target, then prunes what the builds left behind; a build resets `agentic run`'s update check.
func (s *Service) Apply(plan Plan) error {
	opts := plan.Opts
	if err := config.SaveCustomInstalls(s.home, opts.CustomInstalls); err != nil {
		return fmt.Errorf("saving custom installs: %w", err)
	}

	for _, t := range plan.Targets {
		logging.Step(t.Image)
		for _, line := range opts.Overrides() {
			logging.Detail(line)
		}

		if err := s.docker.BuildTool(t.Name, t.Image, opts); err != nil {
			return err
		}

		state, err := config.LoadState(s.home)
		if err == nil {
			err = state.MarkToolChecked(t.Name, time.Now(), s.home)
		}
		if err != nil {
			logging.Warnf("could not record update check for %s: %v", t.Name, err)
		}
	}

	// Best effort: a failed cleanup never fails the build
	_, _ = s.docker.PruneImages()
	return nil
}
