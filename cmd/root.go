// Package cmd provides the Agentic CLI
package cmd

import (
	"fmt"
	"os"
	"slices"

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

var (
	// noDockerCmds lists subcommands that do not require a running Docker daemon.
	noDockerCmds = []string{"completion", "aliases", "version", "upgrade"}
	// noUpdateCmds lists subcommands that skip the automatic update check.
	noUpdateCmds = []string{"completion", "aliases", "upgrade"}
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
	PersistentPreRunE: persistentPreRunE,
}

func init() {
	docker.CLIVersion = version
}

// Execute the Agentic CLI
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// persistentPreRunE is the PersistentPreRunE hook for rootCmd. It checks the
// Docker daemon and, for interactive commands, notifies the user when a newer
// agentic release is available.
func persistentPreRunE(cmd *cobra.Command, args []string) error {
	if err := checkDocker(cmd, args); err != nil {
		return err
	}

	if cmd.Parent() != nil && !slices.Contains(noUpdateCmds, cmd.Name()) {
		maybeNotifyUpdate(toolHome)
	}

	return nil
}

// checkDocker verifies the Docker daemon is reachable before any subcommand
// that needs it runs.
func checkDocker(cmd *cobra.Command, _ []string) error {
	// Bare `agentic` (no subcommand) just shows help — no Docker needed.
	if cmd.Parent() == nil {
		return nil
	}

	// Shell completion generation, `aliases`, `version`, and `upgrade` do
	// not need a running daemon.
	if slices.Contains(noDockerCmds, cmd.Name()) {
		return nil
	}

	return checkDockerDaemon()
}

func rootRun(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}
