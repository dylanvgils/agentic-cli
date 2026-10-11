package run

import (
	"io"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/msb"
	"github.com/dylanvgils/agentic-cli/internal/usecase/sidecar"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolupdate"
	"github.com/dylanvgils/agentic-cli/internal/usecase/update"
)

// Indirects the marketplace calls this package makes, so callers can fake them in tests.
var (
	SyncMarketplaces       = marketplace.Sync
	RecordMarketplaceUsage = marketplace.RecordUsage

	// MsbClient runs tools on RuntimeMsb.
	MsbClient Msb = msb.New()
)

// Docker is the subset of *docker.Client this package uses, including what the sidecar, toolupdate and update usecases it runs need.
type Docker interface {
	sidecar.Docker
	toolupdate.Docker
	update.Docker

	ListAllImages(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error)
	ResolveContainerHome(image string) string
	EnsureNamedVolumes(volumes []string, toolHome, containerHome, chownImage string) error
	EnsureNetwork() error
	SaveImage(image string, w io.Writer) error
	RunContainer(rs docker.RunSpec, toolArgs []string) error
}

// Msb is the subset of *msb.Client this package uses.
type Msb interface {
	LoadImage(r io.Reader) error
	Run(rs docker.RunSpec, toolArgs []string) error
}

// Prompter asks the user to approve what a run needs; a nil error means approved.
type Prompter interface {
	TrustDir(dir string) error
	ApproveSettings(layer config.RCLayer, changed []config.GuardedSetting) error
	OfferToolUpdate(tool, installed, latest string) bool
}
