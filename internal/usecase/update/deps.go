package update

import "github.com/dylanvgils/agentic-cli/internal/docker"

// ListAllImages, InspectImage, UpdateTool, and LatestToolVersion indirect the
// docker calls this package makes, so callers can fake them in tests -
// mirrors the seam convention in internal/cli/deps.go.
var (
	ListAllImages     = docker.ListAllImages
	InspectImage      = docker.InspectImage
	UpdateTool        = docker.UpdateTool
	LatestToolVersion = docker.LatestToolVersion
)
