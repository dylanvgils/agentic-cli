package update

import (
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// LatestToolVersion indirects the upstream version lookup so tests can fake it.
var LatestToolVersion = docker.LatestToolVersion

// Docker is the subset of *docker.Client this package uses.
type Docker interface {
	ListAllImages(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error)
	InspectImage(name string) (*docker.ImageInfo, error)
	UpdateTool(tool, image string, opts tools.BuildOptions) error
}
