package run

import (
	"io"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/sidecar"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolupdate"
	"github.com/dylanvgils/agentic-cli/internal/usecase/update"
)

// Docker is the subset of *docker.Client this package uses, including what the sidecar, toolupdate and update usecases it runs need.
type Docker interface {
	sidecar.Docker
	toolupdate.Docker
	update.Docker

	ListAllImages(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error)
	ResolveContainerHome(image string) string
	EnsureNamedVolumes(rs sandbox.RunSpec, chownImage string) error
	SaveImage(image string, w io.Writer) error
	BuildDockerdImage(image string, parent *docker.ImageInfo) error
	StartSidecars(rs sandbox.RunSpec) (docker.Sidecars, error)
	RunTool(sidecars docker.Sidecars) error
	StopSidecars(sidecars docker.Sidecars)
	Capabilities() sandbox.Capabilities
	ServerVersion() (engine, version string, err error)
}

// Microsandbox is the subset of *microsandbox.Client this package uses.
type Microsandbox interface {
	Capabilities() sandbox.Capabilities
	Available() error
	Version() (string, error)
	LoadImage(r io.Reader, tag string) error
	ImageRefs() ([]string, error)
	RemoveImage(ref string) error
	Run(rs sandbox.RunSpec) error
}

// Prompter asks the user to approve what a run needs; a nil error means approved.
type Prompter interface {
	TrustDir(dir string) error
	ApproveSettings(layer config.RCLayer, changed []config.GuardedSetting) error
	OfferToolUpdate(tool, installed, latest string) bool
}
