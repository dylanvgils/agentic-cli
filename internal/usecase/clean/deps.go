package clean

import "github.com/dylanvgils/agentic-cli/internal/docker"

// Docker is the subset of *docker.Client this package uses.
type Docker interface {
	ListAllImages(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error)
	CleanImage(image string) error
	CleanBaseImages() error
	SweepProxyResources() error
	SweepDindResources(toolHome string) error
	RemoveNetwork() error
}
