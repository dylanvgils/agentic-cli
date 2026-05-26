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
	buildTool          = docker.BuildTool
	updateTool         = docker.UpdateTool
	runContainer       = docker.RunContainer
	ensureNamedVolumes = docker.EnsureNamedVolumes
	inspectImage       = docker.InspectImage
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
	Version:      buildVersion(),
	SilenceUsage: true,
	RunE:         rootRun,
}

func rootRun(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

// Execute the Agentic CLI
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
