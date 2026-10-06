package run

import (
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
)

// Indirects the marketplace calls this package makes, so callers can fake them in tests.
var (
	SyncMarketplaces       = marketplace.Sync
	RecordMarketplaceUsage = marketplace.RecordUsage
)

// Docker is the subset of *docker.Client this package uses.
type Docker interface {
	InspectImage(name string) (*docker.ImageInfo, error)
	ResolveContainerHome(image string) string
	EnsureNamedVolumes(volumes []string, toolHome, containerHome, chownImage string) error
	EnsureNetwork() error
}
