package clean

import (
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/housekeeping"
)

// ListAllImages, CleanImage, CleanBaseImages, SweepProxyResources,
// RemoveNetwork, and pruneAuditLogs indirect the docker/housekeeping calls
// this package makes, so callers can fake them in tests - mirrors the seam
// convention in internal/cli/deps.go.
var (
	ListAllImages       = docker.ListAllImages
	CleanImage          = docker.CleanImage
	CleanBaseImages     = docker.CleanBaseImages
	SweepProxyResources = docker.SweepProxyResources
	RemoveNetwork       = docker.RemoveNetwork
	pruneAuditLogs      = housekeeping.PruneJSONLLogs
)
