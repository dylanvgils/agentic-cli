package toolupdate

import (
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
)

// Docker is the subset of *docker.Client this package uses.
type Docker interface {
	InspectImage(image string) (*docker.ImageInfo, error)
}

// Upstream looks up tools' latest released versions; tools.Upstream does it over the network.
type Upstream interface {
	LatestToolVersion(tool, installedLabel string) (latest string, newer bool, ok bool)
}
