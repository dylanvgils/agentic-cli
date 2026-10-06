package cli

import (
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/git"
	"github.com/dylanvgils/agentic-cli/internal/housekeeping"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/migrate"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/usecase/build"
	"github.com/dylanvgils/agentic-cli/internal/usecase/clean"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolupdate"
	"github.com/dylanvgils/agentic-cli/internal/usecase/update"
)

var (
	migrateRun              = migrate.Run
	checkDockerDaemon       = docker.CheckDaemon
	buildProxyImage         = docker.BuildProxyImage
	buildDindImage          = docker.BuildDindImage
	runContainer            = docker.RunContainer
	inspectImage            = docker.InspectImage
	builtTools              = docker.BuiltTools
	listAllImages           = docker.ListAllImages
	cleanImage              = docker.CleanImage
	pruneImages             = docker.PruneImages
	pruneBuildCache         = docker.PruneBuildCache
	pruneProxyLogs          = housekeeping.PruneProxyLogs
	createVolume            = docker.CreateVolume
	listVolumes             = docker.ListVolumes
	listVolumeNames         = docker.ListVolumeNames
	removeVolume            = docker.RemoveVolume
	listRunningContainers   = docker.ListRunningContainers
	isTerminal              = platform.IsTerminal
	setContext              = docker.SetContext
	listContexts            = docker.ListContexts
	checkGitAvailable       = git.CheckAvailable
	loadMarketplaceRegistry = marketplace.LoadRegistry
	saveMarketplaceRegistry = marketplace.SaveRegistry

	// Transitional until cli holds a single Docker client.
	buildDocker      build.Docker      = docker.Default()
	cleanDocker      clean.Docker      = docker.Default()
	updateDocker     update.Docker     = docker.Default()
	toolupdateDocker toolupdate.Docker = docker.Default()
)
