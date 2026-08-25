package clean

import "github.com/dylanvgils/agentic-cli/internal/docker"

// Indirects the docker calls this package makes, so callers can fake them in tests (seam convention, see internal/cli/deps.go).
var (
	ListAllImages       = docker.ListAllImages
	CleanImage          = docker.CleanImage
	CleanBaseImages     = docker.CleanBaseImages
	SweepProxyResources = docker.SweepProxyResources
	RemoveNetwork       = docker.RemoveNetwork
)
