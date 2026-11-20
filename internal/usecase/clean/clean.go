// Package clean removes agentic's images and shared Docker resources for `agentic clean` and `agentic prune`.
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

// Plan is what Apply removes; SharedResources adds the shared images and network.
type Plan struct {
	Targets         []Target
	SharedResources bool
}

// Scope describes which tool images Resolve should target.
type Scope struct {
	// Tool is the tool to resolve in Namespace, or the one tool an All resolve narrows to (empty means every tool).
	Tool      string
	Namespace string
	All       bool
}

// New returns a Service that cleans agentic state under home, and tool images from Docker and the other runtimes.
func New(home string, d Docker, runtimes ...ImageCleaner) *Service {
	return &Service{docker: d, runtimes: runtimes, home: home}
}

// Resolve returns the plan for scope; a bare All also removes the shared resources.
func (s *Service) Resolve(scope Scope) (Plan, error) {
	if scope.All {
		targets, err := s.resolveAll(scope.Tool)
		return Plan{Targets: targets, SharedResources: scope.Tool == ""}, err
	}

	image, err := tools.ImageName(scope.Tool, scope.Namespace)
	if err != nil {
		return Plan{}, err
	}
	return Plan{Targets: []Target{{Label: image, Image: image}}}, nil
}

// Apply removes plan's images from every runtime, then with SharedResources the shared resources; active runs keep theirs.
func (s *Service) Apply(plan Plan) error {
	if err := s.removeTargets(plan.Targets); err != nil {
		return err
	}
	if !plan.SharedResources {
		return nil
	}

	logging.Separate()
	return s.removeSharedResources()
}

// Prune removes leftovers of interrupted runs, dangling images and, if confirmCache approves, the build cache.
func (s *Service) Prune(confirmCache func() bool) error {
	swept, err := s.docker.SweepRuns(s.home)
	if err != nil {
		return err
	}
	stepRemoved("leftover sidecars", swept.Sidecars)
	stepRemoved("leftover run networks", swept.Networks)

	if err := stepReclaimed("dangling agentic images", s.docker.PruneImages); err != nil {
		return err
	}

	if !confirmCache() {
		logging.Step("build cache (kept)")
		return nil
	}
	return stepReclaimed("build cache", s.docker.PruneBuildCache)
}

// removeTargets removes each target's image, then the custom installs no image uses anymore.
func (s *Service) removeTargets(targets []Target) error {
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

// removeSharedResources removes the base and sidecar images, leftover runs and agentic-net; active runs keep theirs.
func (s *Service) removeSharedResources() error {
	logging.Info("removing shared resources")

	logging.Step("base")
	if err := s.docker.CleanBaseImages(); err != nil {
		return err
	}

	// Interrupted runs go first, so only active runs keep the sidecar images
	if _, err := s.docker.SweepRuns(s.home); err != nil {
		return err
	}
	if err := s.cleanSidecarImages(); err != nil {
		return err
	}

	logging.Step("network")
	return s.docker.RemoveNetwork()
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

// stepImage reports the image under label, marked as kept when a running container uses it; returns inUse.
func stepImage(label string, inUse bool) bool {
	if inUse {
		logging.Stepf("%s (in use, kept)", label)
	} else {
		logging.Step(label)
	}
	return inUse
}

// stepRemoved lists the names removed under label, or "none".
func stepRemoved(label string, names []string) {
	logging.Step(label)
	if len(names) == 0 {
		logging.Detail("none")
	}
	for _, name := range names {
		logging.Detail(name)
	}
}

// stepReclaimed runs prune under label and reports the space it reclaimed, if known.
func stepReclaimed(label string, prune func() (string, error)) error {
	logging.Step(label)
	size, err := prune()
	if err != nil {
		return err
	}

	if size != "" {
		logging.Detailf("reclaimed %s", size)
	}
	return nil
}
