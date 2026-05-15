// Package cmd provides the Agentic CLI
package cmd

import (
	"fmt"
	"os"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/script"
	"github.com/spf13/cobra"
)

var version = "dev"

var (
	runContainer       = docker.RunContainer
	ensureNamedVolumes = docker.EnsureNamedVolumes
	inspectImage       = docker.InspectImage
	cleanImage         = docker.CleanImage
	cleanBaseImages    = docker.CleanBaseImages
)

var rootCmd = &cobra.Command{
	Use:   "agentic-cli",
	Short: "Run agentic coding tools in sandboxed containers",
	Long: `agentic runs AI coding tools (Claude Code, Copilot, OpenCode) in
sandboxed Docker containers with read-only filesystems and dropped capabilities.`,
	Version:      version,
	SilenceUsage: true,
	Args:         cobra.ArbitraryArgs,
	RunE:         rootRun,
}

func rootRun(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	// if command not exist forward to old script
	return script.Delegate("agentic", args)
}

// Execute the Agentic CLI
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
