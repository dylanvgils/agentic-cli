package dashboard

import "github.com/dylanvgils/agentic-cli/internal/docker"

// CheckDaemon, ListAllImages, ListRunningContainers, and ListVolumesInfo
// indirect the docker calls this package makes, so callers can fake them in
// tests - mirrors the seam convention in internal/cli/deps.go.
var (
	CheckDaemon           = docker.CheckDaemon
	ListAllImages         = docker.ListAllImages
	ListRunningContainers = docker.ListRunningContainers
	ListVolumesInfo       = docker.ListVolumesInfo
)
