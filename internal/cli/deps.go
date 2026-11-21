package cli

import (
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/cmdlog"
	"github.com/dylanvgils/agentic-cli/internal/infra/selfupdate"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/build"
	"github.com/dylanvgils/agentic-cli/internal/usecase/clean"
	"github.com/dylanvgils/agentic-cli/internal/usecase/run"
	"github.com/dylanvgils/agentic-cli/internal/usecase/sidecar"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolruns"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolupdate"
	"github.com/dylanvgils/agentic-cli/internal/usecase/update"
	"github.com/dylanvgils/agentic-cli/internal/usecase/upgradecheck"
	"github.com/dylanvgils/agentic-cli/internal/usecase/volumes"
	"github.com/dylanvgils/agentic-cli/internal/usecase/watch"
)

var (
	// commandLog records the docker and msb commands of an `agentic run`, which opens it in the run's log dir.
	commandLog = cmdlog.New()

	// dockerClient is the Docker client every command uses; persistentPreRunE rebuilds it for the resolved context.
	dockerClient dockerAPI = docker.New("", commandLog)

	// msbClient runs tools for `agentic run --runtime microsandbox` and manages their images and volumes.
	msbClient msbAPI = microsandbox.New(commandLog)

	// upstreamClient looks up the tools' latest released versions for `agentic update`.
	upstreamClient update.Upstream = tools.Upstream{}

	// releasesClient fetches and installs agentic's own releases for `agentic upgrade` and the upgrade check.
	releasesClient upgradecheck.Releases = selfupdate.Releases{}

	// watchdogs starts the watchdog that tears each run down once agentic exits.
	watchdogs watchdogStarter = spawnedWatchdogs{}
)

// dockerAPI is the subset of *docker.Client the cli and the usecases it calls use.
type dockerAPI interface {
	build.Docker
	clean.Docker
	update.Docker
	run.Docker
	sidecar.Docker
	toolruns.Docker
	toolupdate.Docker
	volumes.Docker

	Context() string
	CurrentContext() (string, error)
	CheckDaemon() error
	CreateVolume(name string) error
	ListRunningContainers() ([]*docker.ContainerInfo, error)
	ListContexts() ([]string, error)
	Version() (string, error)
}

// msbAPI is the subset of *microsandbox.Client the usecases the cli calls use.
type msbAPI interface {
	run.Microsandbox
	toolruns.Microsandbox
	clean.ImageCleaner
	volumes.Store

	Available() error
	ListRunning() ([]*microsandbox.Sandbox, error)
	Version() (string, error)
}

// watchdogStarter returns the watchdog for runs on a Docker context.
type watchdogStarter interface {
	For(dockerContext string) run.Watchdog
}

// spawnedWatchdogs reruns this executable's watch command as each run's watchdog.
type spawnedWatchdogs struct{}

func (spawnedWatchdogs) For(dockerContext string) run.Watchdog {
	return watch.NewSpawner(watchCmdName, dockerContext)
}
