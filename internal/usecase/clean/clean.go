// Package clean resolves and removes agentic-owned tool images and global Docker resources for `agentic clean`.
package clean

import (
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// Service removes agentic-owned images and shared Docker resources.
type Service struct {
	docker Docker
}

// Target is one image to remove, with the label to report it under.
type Target struct {
	Label string
	Image string
}

// Scope describes which tool images Resolve should target.
type Scope struct {
	// Names is the tool name list to resolve in scoped mode, already expanded to every known tool when none was given.
	Names []string
	// FilterTool narrows an --all resolve to one tool; empty means every tool.
	FilterTool string
	Namespace  string
	All        bool
}

// New returns a Service that talks to Docker through d.
func New(d Docker) *Service {
	return &Service{docker: d}
}

// Resolve returns the clean targets for scope: every agentic image across all namespaces (All), or the named tools in one namespace.
func (s *Service) Resolve(scope Scope) ([]Target, error) {
	if scope.All {
		return s.resolveAll(scope.FilterTool)
	}
	return resolveScoped(scope.Names, scope.Namespace)
}

// Apply removes each target's image, reporting progress.
func (s *Service) Apply(targets []Target) error {
	for _, t := range targets {
		logging.Step(t.Label)
		if err := s.docker.CleanImage(t.Image); err != nil {
			return err
		}
	}

	return nil
}

// GlobalResources removes agentic's shared Docker resources: base and sidecar images, leftover sidecar resources, and agentic-net.
func (s *Service) GlobalResources(toolHome string) error {
	logging.Infof("removing shared resources: base images, %s, %s, sidecars, %s", tools.ProxyImage, tools.DindImage, docker.NetworkName)

	logging.Step("base")
	if err := s.docker.CleanBaseImages(); err != nil {
		return err
	}

	logging.Step(tools.ProxyImage)
	if err := s.docker.CleanImage(tools.ProxyImage); err != nil {
		return err
	}
	logging.Step(tools.DindImage)
	if err := s.docker.CleanImage(tools.DindImage); err != nil {
		return err
	}
	// Sidecars may sit on a proxy network, so they go first
	if err := s.docker.SweepDindResources(toolHome); err != nil {
		return err
	}
	if err := s.docker.SweepProxyResources(); err != nil {
		return err
	}

	logging.Step("network")
	return s.docker.RemoveNetwork()
}

func (s *Service) resolveAll(filterTool string) ([]Target, error) {
	var filters []docker.ImageFilter
	if filterTool != "" {
		filters = append(filters, docker.ToolFilter(filterTool))
	}

	images, err := s.docker.ListAllImages(filters...)
	if err != nil {
		return nil, err
	}

	var targets []Target
	for _, info := range images {
		if _, ok := tools.Configs[info.Tool]; !ok {
			continue
		}
		targets = append(targets, Target{Label: info.Namespace + "/" + info.Tool, Image: info.Image})
	}

	return targets, nil
}

func resolveScoped(names []string, namespace string) ([]Target, error) {
	targets := make([]Target, 0, len(names))

	for _, name := range names {
		image, err := tools.ImageName(name, namespace)
		if err != nil {
			return nil, err
		}
		targets = append(targets, Target{Label: image, Image: image})
	}

	return targets, nil
}
