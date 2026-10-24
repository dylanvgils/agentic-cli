package build

import "github.com/dylanvgils/agentic-cli/internal/image/tools"

// Docker is the subset of *docker.Client this package uses.
type Docker interface {
	BuildTool(tool, image string, opts tools.BuildOptions) error
	PruneImages() (string, error)
}
