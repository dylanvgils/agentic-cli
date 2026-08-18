package run

import (
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
)

// EnsureNamedVolumes, EnsureNetwork, SyncMarketplaces, and
// RecordMarketplaceUsage indirect the docker/marketplace calls Build makes,
// so callers can fake them in tests - mirrors the seam convention in
// internal/cli/deps.go.
var (
	EnsureNamedVolumes     = docker.EnsureNamedVolumes
	EnsureNetwork          = docker.EnsureNetwork
	SyncMarketplaces       = marketplace.Sync
	RecordMarketplaceUsage = marketplace.RecordUsage
)
