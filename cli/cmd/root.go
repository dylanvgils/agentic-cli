// Package cmd provides the Agentic CLI
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"slices"

	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/script"
	"github.com/spf13/cobra"
)

var toolHome string

var rootCmd = &cobra.Command{
	Use:   "agentic",
	Short: "Run agentic coding tools in sandboxed containers",
	Long: `agentic runs AI coding tools (Claude Code, Copilot, OpenCode) in
sandboxed Docker containers with read-only filesystems and dropped capabilities.`,
	SilenceUsage:       true,
	Args:               cobra.ArbitraryArgs,
	DisableFlagParsing: true,
	RunE:               rootRun,
}

func rootRun(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}
	if !slices.Contains(validTools, args[0]) {
		return delegateToShell(os.Args[1:])
	}
	return runTool(runToolCmd, args)
}

// Execute the Agentic CLI
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	defaultHome := platform.ToolHomeDefault()
	if env := os.Getenv("AGENTIC_HOME"); env != "" {
		defaultHome = env
	}

	rootCmd.PersistentFlags().StringVar(&toolHome, "home", defaultHome,
		"agentic data directory (overrides $AGENTIC_HOME)")
}

func ToolHome() string {
	return toolHome
}

func delegateToShell(args []string) error {
	scriptPath := script.FindScript("agentic")

	cmd := exec.Command("bash", append([]string{scriptPath}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	return nil
}
