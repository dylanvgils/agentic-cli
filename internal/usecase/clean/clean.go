// Package clean resolves and removes agentic-owned tool images and global Docker resources for `agentic clean`.
package clean

import (
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
)

// Service removes agentic-owned images from every runtime and shared Docker resources.
type Service struct {
	docker   Docker
	runtimes []ImageCleaner // other runtimes holding copies of the tool images, cleaned best-effort
	home     string         // agentic data dir, for the custom installs store and sidecar state
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

// New returns a Service that cleans agentic state under home, and tool images from Docker and the other runtimes.
func New(home string, d Docker, runtimes ...ImageCleaner) *Service {
	return &Service{docker: d, runtimes: runtimes, home: home}
}

// Resolve returns the clean targets for scope: every agentic image across all namespaces (All), or the named tools in one namespace.
func (s *Service) Resolve(scope Scope) ([]Target, error) {
	if scope.All {
		return s.resolveAll(scope.FilterTool)
	}
	return resolveScoped(scope.Names, scope.Namespace)
}

// Apply removes each target's image from Docker and every other runtime, reporting progress, then the stored custom installs no image uses anymore.
// Images an active run uses are kept.
func (s *Service) Apply(targets []Target) error {
	inUse, err := s.imagesInUse()
	if err != nil {
		return err
	}
	before, beforeErr := s.customInstallsHashes()

	for _, t := range targets {
		if stepImage(t.Label, inUse[t.Image]) {
			continue
		}
		// The dockerd image is built FROM the tool image, so it goes first
		for _, image := range []string{tools.DockerdImage(t.Image), t.Image} {
			if err := s.removeImage(image); err != nil {
				return err
			}
		}
	}

	if beforeErr == nil {
		s.pruneCustomInstalls(before)
	}
	return nil
}

// removeImage removes image from Docker, then best-effort from every other runtime.
func (s *Service) removeImage(image string) error {
	if err := s.docker.CleanImage(image); err != nil {
		return err
	}
	for _, r := range s.runtimes {
		if err := r.CleanImage(image); err != nil {
			logging.Warnf("kept %s: %v", image, err)
		}
	}
	return nil
}

// GlobalResources removes agentic's shared Docker resources: base and sidecar images, leftover sidecars and run
// networks, and agentic-net. Active runs keep their sidecars, networks and images.
func (s *Service) GlobalResources() error {
	logging.Infof("removing shared resources")

	logging.Step("base")
	if err := s.docker.CleanBaseImages(); err != nil {
		return err
	}

	// Orphaned sidecars go first, so only active runs keep the sidecar images
	if err := s.sweepSidecars(); err != nil {
		return err
	}
	if err := s.cleanSidecarImages(); err != nil {
		return err
	}

	// Sidecars sit on a run network, so they go first
	if err := s.docker.SweepRunNetworks(); err != nil {
		return err
	}

	logging.Step("network")
	return s.docker.RemoveNetwork()
}

// sweepSidecars removes the sidecars no active run uses.
func (s *Service) sweepSidecars() error {
	if err := s.docker.SweepDindResources(s.home); err != nil {
		return err
	}
	return s.docker.SweepProxyResources()
}

// cleanSidecarImages removes the proxy and dind images, keeping those an active run uses.
func (s *Service) cleanSidecarImages() error {
	inUse, err := s.imagesInUse()
	if err != nil {
		return err
	}

	for _, image := range []string{tools.ProxyImage, tools.DindImage} {
		if stepImage(image, inUse[image]) {
			continue
		}
		if err := s.docker.CleanImage(image); err != nil {
			return err
		}
	}
	return nil
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

// imagesInUse returns the images of agentic's running containers.
func (s *Service) imagesInUse() (map[string]bool, error) {
	containers, err := s.docker.ListRunningContainers()
	if err != nil {
		return nil, err
	}

	inUse := make(map[string]bool, len(containers))
	for _, c := range containers {
		inUse[c.Image] = true
	}
	return inUse, nil
}

// customInstallsHashes returns the custom installs hashes recorded on agentic's images.
func (s *Service) customInstallsHashes() (map[string]bool, error) {
	images, err := s.docker.ListAllImages()
	if err != nil {
		return nil, err
	}

	hashes := make(map[string]bool)
	for _, info := range images {
		if info.CustomInstallsHash != "" {
			hashes[info.CustomInstallsHash] = true
		}
	}
	return hashes, nil
}

// pruneCustomInstalls deletes the stored custom installs of hashes in before that no image records anymore. Best-effort:
// only hashes seen in this Docker context are touched, so other contexts' images keep theirs.
func (s *Service) pruneCustomInstalls(before map[string]bool) {
	after, err := s.customInstallsHashes()
	if err != nil {
		return
	}

	for hash := range before {
		if !after[hash] {
			_ = config.RemoveCustomInstalls(s.home, hash)
		}
	}
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

// stepImage reports the image under label, marked as kept when a running container uses it; returns inUse.
func stepImage(label string, inUse bool) bool {
	if inUse {
		logging.Stepf("%s (in use, kept)", label)
	} else {
		logging.Step(label)
	}
	return inUse
}
