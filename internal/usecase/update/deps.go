package update

import "github.com/dylanvgils/agentic-cli/internal/docker"

// Indirects the docker calls this package makes, so callers can fake them in tests.
var (
	ListAllImages     = docker.ListAllImages
	InspectImage      = docker.InspectImage
	UpdateTool        = docker.UpdateTool
	LatestToolVersion = docker.LatestToolVersion
)
