package clean

import "github.com/dylanvgils/agentic-cli/internal/docker"

// ListAllImages, CleanImage, CleanBaseImages, SweepProxyResources, and
// RemoveNetwork indirect the docker calls this package makes, so callers
// can fake them in tests - mirrors the seam convention in internal/cli/deps.go.
var (
	ListAllImages       = docker.ListAllImages
	CleanImage          = docker.CleanImage
	CleanBaseImages     = docker.CleanBaseImages
	SweepProxyResources = docker.SweepProxyResources
	RemoveNetwork       = docker.RemoveNetwork
)
