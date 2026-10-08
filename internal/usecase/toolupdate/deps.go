package toolupdate

import (
	"github.com/dylanvgils/agentic-cli/internal/docker"
)

// Indirects the non-Docker calls Check makes, so callers can fake them in tests.
var LatestToolVersion func(tool, installedLabel string) (string, bool, bool) = docker.LatestToolVersion

// Docker is the subset of *docker.Client this package uses.
type Docker interface {
	InspectImage(image string) (*docker.ImageInfo, error)
}
