// Package cmd provides the Agentic CLI
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/spf13/cobra"
)

var (
	version       = "dev"
	commit        = ""
	buildDate     = ""
	installMethod = ""
)

var (
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
	Short: "Run agentic coding tools in sandboxed containers",
	Long: `Agentic runs AI coding tools (Claude Code, Copilot, OpenCode) in
sandboxed Docker containers with read-only filesystems and dropped capabilities.`,
	Version:      buildVersion(),
	SilenceUsage: true,
	RunE:         rootRun,
}

func rootRun(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

func buildVersion() string {
	var meta []string

	if commit != "" {
		meta = append(meta, commit)
	}
	if buildDate != "" {
		meta = append(meta, buildDate)
	}
	if installMethod != "" {
		meta = append(meta, installMethod)
	}

	if len(meta) == 0 {
		return version
	}
	return version + " (" + strings.Join(meta, ", ") + ")"
}

// Execute the Agentic CLI
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
