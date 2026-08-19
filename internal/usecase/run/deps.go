package run

import (
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
)

// Indirections into docker/marketplace calls so callers can fake them in tests - mirrors internal/cli/deps.go.
var (
	EnsureNamedVolumes     = docker.EnsureNamedVolumes
	EnsureNetwork          = docker.EnsureNetwork
	SyncMarketplaces       = marketplace.Sync
	RecordMarketplaceUsage = marketplace.RecordUsage
	InspectImage           = docker.InspectImage
)
