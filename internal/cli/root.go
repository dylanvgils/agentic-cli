// Package cli provides the Agentic CLI
package cli

import (
	"fmt"
	"os"
	"slices"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/housekeeping"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/spf13/cobra"
)

var (
	checkDockerDaemon     = docker.CheckDaemon
	buildTool             = docker.BuildTool
	buildProxyImage       = docker.BuildProxyImage
	updateTool            = docker.UpdateTool
	runContainer          = docker.RunContainer
	ensureNamedVolumes    = docker.EnsureNamedVolumes
	inspectImage          = docker.InspectImage
	builtTools            = docker.BuiltTools
	listAllImages         = docker.ListAllImages
	cleanImage            = docker.CleanImage
	cleanBaseImages       = docker.CleanBaseImages
	pruneImages           = docker.PruneImages
	pruneBuildCache       = docker.PruneBuildCache
	ensureNetwork         = docker.EnsureNetwork
	removeNetwork         = docker.RemoveNetwork
	sweepProxyResources   = docker.SweepProxyResources
	pruneProxyLogs        = housekeeping.PruneProxyLogs
	createVolume          = docker.CreateVolume
	listVolumes           = docker.ListVolumes
	listVolumeNames       = docker.ListVolumeNames
	removeVolume          = docker.RemoveVolume
	listRunningContainers = docker.ListRunningContainers
	isTerminal            = platform.IsTerminal
	setContext            = docker.SetContext
	listContexts          = docker.ListContexts
	syncMarketplaces      = marketplace.Sync
	checkGitAvailable     = marketplace.CheckGitAvailable
	pruneMarketplaces     = marketplace.Prune
)

var (
	// noDockerCmds lists subcommands that do not require a running Docker daemon.
	noDockerCmds = []string{"completion", "aliases", "version", "upgrade", "status"}
	// noUpdateCmds lists subcommands that skip the automatic update check.
	noUpdateCmds = []string{"completion", "aliases", "upgrade"}
)

var rootCmd = &cobra.Command{
	Use:   "agentic",
	Short: "Run agentic coding tools in isolated containers",
	Long: `Agentic runs AI coding tools (Claude Code, Copilot, OpenCode) in
isolated Docker containers with read-only filesystems and dropped capabilities.`,
	Version:           buildinfo.Version,
	SilenceUsage:      true,
	SilenceErrors:     true,
	RunE:              rootRun,
	PersistentPreRunE: persistentPreRunE,
}

func init() {
	rootCmd.PersistentFlags().String("docker-context", "",
		"Docker context to use (overrides .agenticrc.toml and agentic.json)")
	_ = rootCmd.RegisterFlagCompletionFunc("docker-context", dockerContextsFunc)
}

// Execute the Agentic CLI
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// persistentPreRunE is the PersistentPreRunE hook for rootCmd. It resolves the
// active Docker context, checks the Docker daemon, and, for interactive
// commands, notifies the user when a newer agentic release is available.
func persistentPreRunE(cmd *cobra.Command, args []string) error {
	resolveContext(cmd)

	if err := checkDocker(cmd, args); err != nil {
		return err
	}

	if cmd.Parent() != nil && !inCommandChain(cmd, noUpdateCmds) {
		maybeNotifyUpdate(toolHome)
	}

	return nil
}

// resolveContext resolves the active Docker context from the --docker-context
// flag, .agenticrc.toml, and agentic.json, then sets it for all subsequent
// docker invocations in this process. If none of those are set, the docker
// CLI's own context resolution (including its DOCKER_CONTEXT env var) applies
// unchanged.
func resolveContext(cmd *cobra.Command) {
	rc, err := config.FindAndLoadFromCwd()
	if err != nil {
		rc = &config.AgenticRC{}
	}

	flagVal, _ := cmd.Flags().GetString("docker-context")
	setContext(config.ResolveDockerContext(flagVal, rc, toolHome))
}

// checkDocker verifies the Docker daemon is reachable before any subcommand
// that needs it runs.
func checkDocker(cmd *cobra.Command, _ []string) error {
	// Bare `agentic` (no subcommand) just shows help - no Docker needed.
	if cmd.Parent() == nil {
		return nil
	}

	if inCommandChain(cmd, noDockerCmds) {
		return nil
	}

	return checkDockerDaemon()
}

// inCommandChain reports whether cmd or any of its ancestors (up to but not
// including root) has a name in names. Handles subcommands like
// `completion bash`, where cmd.Name() is "bash" but the parent is "completion".
func inCommandChain(cmd *cobra.Command, names []string) bool {
	for command := cmd; command.Parent() != nil; command = command.Parent() {
		if slices.Contains(names, command.Name()) {
			return true
		}
	}
	return false
}

// pruneResources silently removes agentic-owned dangling images and build cache.
func pruneResources() {
	_ = pruneImages()
	_ = pruneBuildCache()
}

func rootRun(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}
