package cmd

import (
	"fmt"
	"os"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/spf13/cobra"
)

var validTools = []string{"claude", "copilot", "opencode"}

var (
	toolHome     string
	extraVolumes []string
	pidsLimit    string
	cpus         string
	memory       string
)

var runContainer = docker.RunContainer

func init() {
	rootCmd.AddCommand(runToolCmd)

	defaultHome := platform.ToolHomeDefault()
	if env := os.Getenv("AGENTIC_HOME"); env != "" {
		defaultHome = env
	}

	runToolCmd.Flags().StringVar(&toolHome, "home", defaultHome,
		"agentic data directory (overrides $AGENTIC_HOME)")
	runToolCmd.Flags().StringArrayVarP(&extraVolumes, "volume", "v", nil,
		"additional volume mount (format: host:container[:options]); repeatable")
	runToolCmd.Flags().StringVar(&pidsLimit, "pids-limit", "", "container PID limit")
	runToolCmd.Flags().StringVar(&cpus, "cpus", "", "CPU limit")
	runToolCmd.Flags().StringVar(&memory, "memory", "", "memory limit")
	runToolCmd.Flags().SetInterspersed(false)
}

var runToolCmd = &cobra.Command{
	Use:       "run [flags] <tool> [args...]",
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
	imageName := fmt.Sprintf("agentic-%s", toolName)

	skipEntrypoint := len(toolArgs) > 0 && toolArgs[0] == "--"
	if skipEntrypoint {
		toolArgs = toolArgs[1:]
	}

	cwd, _ := os.Getwd()
	rc := config.FindAndLoad(cwd)

	volumes := append(extraVolumes, rc.ExtraMounts...)
	if pidsLimit == "" {
		pidsLimit = rc.PidsLimit
	}
	if cpus == "" {
		cpus = rc.CPUs
	}
	if memory == "" {
		memory = rc.Memory
	}

	rs := docker.RunSpec{
		Image:          imageName,
		ToolHome:       toolHome,
		Volumes:        volumes,
		SkipEntrypoint: skipEntrypoint,
		PidsLimit:      pidsLimit,
		CPUs:           cpus,
		Memory:         memory,
	}

	return runContainer(rs, toolArgs)
}
