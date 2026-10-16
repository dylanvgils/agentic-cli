package clean

import "github.com/dylanvgils/agentic-cli/internal/docker"

// ImageCleaner removes a runtime's own copies of the tool images; clean calls it on every runtime.
type ImageCleaner interface {
	CleanImage(image string) error
}

// Docker is the subset of *docker.Client this package uses.
type Docker interface {
	ImageCleaner

	ListAllImages(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error)
	ListRunningContainers() ([]*docker.ContainerInfo, error)
	CleanBaseImages() error
	SweepProxyResources() error
	SweepDindResources(toolHome string) error
	SweepRunNetworks() error
	RemoveNetwork() error
}
