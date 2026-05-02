package cmd

import (
	"fmt"
	"os"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/spf13/cobra"
)

var validTools = []string{"claude", "copilot", "opencode"}

func init() {
	rootCmd.AddCommand(runToolCmd)
}

var runToolCmd = &cobra.Command{
	Use:       "run <tool> [args...]",
	Short:     "Run a tool container",
	Long:      `Run a tool container in the current directory.`,
	Args:      cobra.ArbitraryArgs,
	ValidArgs: validTools,
	RunE:      runTool,
	Hidden:    false,
}

func runTool(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	toolName := args[0]
	toolArgs := args[1:]

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	rs := docker.RunSpec{
		Image:    toolName,
		ToolHome: ToolHome(),
	}

	fmt.Printf("WorkingDir: %s", cwd)

	return docker.RunContainer(rs, toolArgs)
}
