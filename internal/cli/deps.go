package cli

import (
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/git"
	"github.com/dylanvgils/agentic-cli/internal/housekeeping"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/migrate"
	"github.com/dylanvgils/agentic-cli/internal/platform"
)

var (
	migrateRun              = migrate.Run
	checkDockerDaemon       = docker.CheckDaemon
	buildProxyImage         = docker.BuildProxyImage
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
)
