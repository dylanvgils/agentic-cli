package sidecar

import (
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// Docker is the subset of *docker.Client this package uses.
type Docker interface {
	InspectImage(name string) (*docker.ImageInfo, error)
	BuildProxyImage(image, version, sourceDir string, opts tools.BuildOptions) error
	BuildDindImage(image string, opts tools.BuildOptions) error
	PruneDangling() error
}
