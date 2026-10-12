package run

import (
	"io"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/sidecar"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolupdate"
	"github.com/dylanvgils/agentic-cli/internal/usecase/update"
)

// Indirects the marketplace calls this package makes, so callers can fake them in tests.
var (
	SyncMarketplaces       = marketplace.Sync
	RecordMarketplaceUsage = marketplace.RecordUsage
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
	RunContainer(rs sandbox.RunSpec, toolArgs []string) error
}

// Microsandbox is the subset of *microsandbox.Client this package uses.
type Microsandbox interface {
	Available() error
	LoadImage(r io.Reader, tag string) error
	ImageRefs() ([]string, error)
	RemoveImage(ref string) error
	Run(rs sandbox.RunSpec, toolArgs []string) error
}

// Prompter asks the user to approve what a run needs; a nil error means approved.
type Prompter interface {
	TrustDir(dir string) error
	ApproveSettings(layer config.RCLayer, changed []config.GuardedSetting) error
	OfferToolUpdate(tool, installed, latest string) bool
}
