// Package cli provides the Agentic CLI
package cli

import (
	"fmt"
	"os"
	"slices"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
	"github.com/dylanvgils/agentic-cli/internal/usecase/run"
	"github.com/dylanvgils/agentic-cli/internal/usecase/upgradecheck"
	"github.com/spf13/cobra"
)

var (
	// noDockerCmds lists subcommands that do not require a running Docker daemon.
	noDockerCmds = []string{"completion", "aliases", "version", "upgrade", "status", "marketplaces", "instructions"}
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

// persistentPreRunE is rootCmd's PersistentPreRunE: resolves the Docker context, checks Docker/git, and offers an upgrade if available.
func persistentPreRunE(cmd *cobra.Command, args []string) error {
	resolveContext(cmd)

	if err := checkDocker(cmd, args); err != nil {
		return err
	}

	if err := checkGit(cmd, args); err != nil {
		return err
	}

	if cmd.Parent() != nil && !inCommandChain(cmd, noUpdateCmds) {
		upgradecheck.Check(toolHome)
	}

	return nil
}

// resolveContext resolves the active Docker context from --docker-context, .agenticrc.toml, or agentic.json and sets it for this process; if none are set, the docker CLI's own context resolution applies.
func resolveContext(cmd *cobra.Command) {
	rc, err := config.FindAndLoadFromCwd()
	if err != nil {
		rc = &config.AgenticRC{}
	}

	flagVal, _ := cmd.Flags().GetString("docker-context")
	setContext(resolve.DockerContext(flagVal, rc, toolHome))
}

// checkDocker verifies the Docker daemon is reachable before any subcommand that needs it runs.
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

// checkGit verifies git is on the host PATH before `run` starts a tool that needs it to sync configured marketplaces.
func checkGit(cmd *cobra.Command, args []string) error {
	if cmd.Name() != "run" || len(args) == 0 {
		return nil
	}

	toolConfig, ok := tools.Configs[args[0]]
	if !ok {
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	rc, err := config.FindAndLoad(cwd)
	if err != nil {
		return nil
	}

	if !run.ToolNeedsMarketplaceSync(toolConfig, rc, args[0]) {
		return nil
	}

	return checkGitAvailable()
}

// inCommandChain reports whether cmd or any ancestor up to (not including) root has a name in names (e.g. matches "completion" for `completion bash`).
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
