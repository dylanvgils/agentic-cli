package run

import (
	"io"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
)

// Docker is the subset of *docker.Client this package uses.
type Docker interface {
	InspectImage(name string) (*docker.ImageInfo, error)
	ListAllImages(filters ...docker.ImageFilter) ([]*docker.ImageInfo, error)
	ResolveContainerHome(image string) string
	EnsureNamedVolumes(rs sandbox.RunSpec, chownImage string) error
	SaveImage(image string, w io.Writer) error
	BuildDockerdImage(image string, parent *docker.ImageInfo) error
	StartSidecars(rs sandbox.RunSpec, proxyLog string) (docker.Sidecars, error)
	RunTool(sidecars docker.Sidecars) error
	StopSidecars(sidecars docker.Sidecars)
	Teardown(toolHome string, id sandbox.RunID) error
	StartPublishedProxy(rs sandbox.RunSpec, proxyLog string, owner sandbox.Runtime) (docker.PublishedProxy, error)
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
	Run(rs sandbox.RunSpec, route *microsandbox.ProxyRoute) error
	Teardown(id sandbox.RunID) error
}

// ToolUpdate names the tool image a run offers an update for, and where its check state and config live.
type ToolUpdate struct {
	Home  string
	RC    *config.AgenticRC
	Tool  string
	Image string
}

// ToolUpdates offers a due tool update through confirm and applies it when confirm agrees.
type ToolUpdates interface {
	Offer(u ToolUpdate, confirm func(tool, installed, latest string) bool) error
}

// SidecarImages builds the proxy and dind sidecar images a run needs when they're missing.
type SidecarImages interface {
	Ensure(proxy, dind bool, registry string) error
}

// Prompter asks the user to approve what a run needs; a nil error means approved.
type Prompter interface {
	TrustDir(dir string) error
	ApproveSettings(layer config.RCLayer, changed []config.GuardedSetting) error
	OfferToolUpdate(tool, installed, latest string) bool
}
