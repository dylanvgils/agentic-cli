// Package clean resolves and removes agentic-owned tool images and global
// Docker resources for `agentic clean`.
package clean

import (
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// Target is one image to remove, with the label to report it under.
type Target struct {
	Label string
	Image string
}

// Scope describes which tool images Resolve should target.
type Scope struct {
	// Names is the tool name list to resolve in scoped mode - already
	// expanded from CLI args to every known tool when no tool was given.
	Names []string
	// FilterTool narrows an --all resolve to one tool; empty means every tool.
	FilterTool string
	Namespace  string
	All        bool
}

// Resolve returns the clean targets for scope: either every agentic image
// found across all namespaces (All) or the named tools in a single namespace.
func Resolve(scope Scope) ([]Target, error) {
	if scope.All {
		return resolveAll(scope.FilterTool)
	}
	return resolveScoped(scope.Names, scope.Namespace)
}

// Apply removes each target's image, reporting progress.
func Apply(targets []Target) error {
	for _, t := range targets {
		logging.Step(t.Label)
		if err := CleanImage(t.Image); err != nil {
			return err
		}
	}

	return nil
}

// GlobalResources removes agentic's shared, non-tool-specific Docker
// resources: base images, the proxy image, other proxy resources, and the
// agentic-net network.
func GlobalResources() error {
	logging.Step("base")
	if err := CleanBaseImages(); err != nil {
		return err
	}

	if err := cleanProxyImage(); err != nil {
		return err
	}
	if err := SweepProxyResources(); err != nil {
		return err
	}

	logging.Step("network")
	return RemoveNetwork()
}

// cleanProxyImage removes the proxy image. Duplicates the equivalent
// one-liner in internal/cli/proxy.go (shared there by `agentic proxy
// clean`) since this package can't depend on internal/cli.
func cleanProxyImage() error {
	logging.Step(tools.ProxyImage)
	return CleanImage(tools.ProxyImage)
}

func resolveAll(filterTool string) ([]Target, error) {
	var filters []docker.ImageFilter
	if filterTool != "" {
		filters = append(filters, docker.ToolFilter(filterTool))
	}

	images, err := ListAllImages(filters...)
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
