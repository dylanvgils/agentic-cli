package dashboard

import "github.com/dylanvgils/agentic-cli/internal/docker"

// Indirects the docker calls this package makes, so callers can fake them in tests - mirrors internal/cli/deps.go.
var (
	CheckDaemon           = docker.CheckDaemon
	ListAllImages         = docker.ListAllImages
	ListRunningContainers = docker.ListRunningContainers
	ListVolumesInfo       = docker.ListVolumesInfo
	VolumeSizes           = docker.VolumeSizes
)
