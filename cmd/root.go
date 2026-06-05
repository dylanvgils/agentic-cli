// Package cmd provides the Agentic CLI
package cmd

import (
	"fmt"
	"os"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/spf13/cobra"
)

var (
	checkDockerDaemon  = docker.CheckDaemon
	buildTool          = docker.BuildTool
	updateTool         = docker.UpdateTool
	runContainer       = docker.RunContainer
	ensureNamedVolumes = docker.EnsureNamedVolumes
	inspectImage       = docker.InspectImage
	builtTools         = docker.BuiltTools
	listAllImages      = docker.ListAllImages
	cleanImage         = docker.CleanImage
	cleanBaseImages    = docker.CleanBaseImages
	pruneImages        = docker.PruneImages
	createVolume       = docker.CreateVolume
	listVolumes        = docker.ListVolumes
	listVolumeNames    = docker.ListVolumeNames
	removeVolume       = docker.RemoveVolume
	isTerminal         = platform.IsTerminal
)

var rootCmd = &cobra.Command{
	Use:   "agentic",
	Short: "Run agentic coding tools in isolated containers",
	Long: `Agentic runs AI coding tools (Claude Code, Copilot, OpenCode) in
isolated Docker containers with read-only filesystems and dropped capabilities.`,
	Version:           version,
	SilenceUsage:      true,
	SilenceErrors:     true,
	RunE:              rootRun,
	PersistentPreRunE: checkDocker,
}

// Execute the Agentic CLI
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// checkDocker is the PersistentPreRunE hook that verifies the Docker daemon is
// reachable before any subcommand that needs it runs.
func checkDocker(cmd *cobra.Command, _ []string) error {
	// Bare `agentic` (no subcommand) just shows help — no Docker needed.
	if cmd.Parent() == nil {
		return nil
	}

	// Shell completion generation and `aliases` do not need a running daemon.
	// (`aliases` ignores the error from builtTools and prints no aliases when Docker is unavailable.)
	if name := cmd.Name(); name == "completion" || name == "aliases" || name == "version" {
		return nil
	}

	return checkDockerDaemon()
}

func rootRun(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}
