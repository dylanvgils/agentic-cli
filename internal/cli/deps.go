package cli

import (
	"io"
	"os"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/selfupdate"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
	"github.com/dylanvgils/agentic-cli/internal/runtime/microsandbox"
	"github.com/dylanvgils/agentic-cli/internal/usecase/build"
	"github.com/dylanvgils/agentic-cli/internal/usecase/clean"
	"github.com/dylanvgils/agentic-cli/internal/usecase/run"
	"github.com/dylanvgils/agentic-cli/internal/usecase/sidecar"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolruns"
	"github.com/dylanvgils/agentic-cli/internal/usecase/update"
	"github.com/dylanvgils/agentic-cli/internal/usecase/upgradecheck"
	"github.com/dylanvgils/agentic-cli/internal/usecase/volumes"
)

var (
	// stdin is where every confirmation prompt reads its answer.
	stdin io.Reader = os.Stdin

	// dockerClient is the Docker client every command uses; persistentPreRunE rebuilds it for the resolved context.
	dockerClient dockerAPI = docker.New("")

	// msbClient runs tools for `agentic run --runtime microsandbox` and manages their images and volumes.
	msbClient msbAPI = microsandbox.New()

	// upstreamClient looks up the tools' latest released versions for `agentic update`.
	upstreamClient update.Upstream = tools.Upstream{}

	// releasesClient fetches and installs agentic's own releases for `agentic upgrade` and the upgrade check.
	releasesClient upgradecheck.Releases = selfupdate.Releases{}
)

// dockerAPI is the subset of *docker.Client the cli and the usecases it calls use.
type dockerAPI interface {
	build.Docker
	clean.Docker
	update.Docker
	run.Docker
	sidecar.Docker
	toolruns.Docker
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
