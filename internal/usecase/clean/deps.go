package clean

import "github.com/dylanvgils/agentic-cli/internal/docker"

// Runtime is a sandbox runtime that holds its own copy of the tool images.
type Runtime interface {
	CleanImage(image string) error
}

// Docker is the subset of *docker.Client this package uses.
type Docker interface {
	Runtime

	ListAllImages(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error)
	CleanBaseImages() error
	SweepProxyResources() error
	SweepDindResources(toolHome string) error
	RemoveNetwork() error
}
