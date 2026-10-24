package update

import (
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
)

// Docker is the subset of *docker.Client this package uses.
type Docker interface {
	ListAllImages(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error)
	InspectImage(name string) (*docker.ImageInfo, error)
	BuildTool(tool, image string, opts tools.BuildOptions) error
	RestampImage(image string, info docker.ImageInfo)
	PruneImages() (string, error)
}

// Upstream looks up tools' latest released versions; tools.Upstream does it over the network.
type Upstream interface {
	LatestToolVersion(tool, installedLabel string) (latest string, newer bool, ok bool)
}
