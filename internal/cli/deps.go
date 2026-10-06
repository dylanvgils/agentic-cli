package cli

import (
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/git"
	"github.com/dylanvgils/agentic-cli/internal/housekeeping"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/migrate"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/build"
	"github.com/dylanvgils/agentic-cli/internal/usecase/clean"
	"github.com/dylanvgils/agentic-cli/internal/usecase/run"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolupdate"
	"github.com/dylanvgils/agentic-cli/internal/usecase/update"
)

var (
	migrateRun              = migrate.Run
	pruneProxyLogs          = housekeeping.PruneProxyLogs
	isTerminal              = platform.IsTerminal
	checkGitAvailable       = git.CheckAvailable
	loadMarketplaceRegistry = marketplace.LoadRegistry
	saveMarketplaceRegistry = marketplace.SaveRegistry

	// newDockerClient builds the client for the resolved Docker context.
	newDockerClient = func(ctx string) dockerAPI { return docker.New(ctx) }

	// dockerClient is the Docker client every command uses; persistentPreRunE rebuilds it for the resolved context.
	dockerClient dockerAPI = docker.New("")
)

// dockerAPI is the subset of *docker.Client the cli and the usecases it calls use.
type dockerAPI interface {
	build.Docker
	clean.Docker
	update.Docker
	toolupdate.Docker
	run.Docker

	Context() string
	CheckDaemon() error
	BuildProxyImage(image, version, sourceDir string, opts tools.BuildOptions) error
	BuildDindImage(image string, opts tools.BuildOptions) error
	RunContainer(rs docker.RunSpec, toolArgs []string) error
	BuiltTools() (map[string]bool, error)
	PruneImages() error
	PruneBuildCache() error
	CreateVolume(name string) error
	ListVolumes() (string, error)
	ListVolumeNames() ([]string, error)
	RemoveVolume(name string) error
	ListRunningContainers() ([]*docker.ContainerInfo, error)
	ListContexts() ([]string, error)
}
