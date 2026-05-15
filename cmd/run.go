package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)

var (
	toolHome     string
	extraVolumes []string
	pidsLimit    string
	cpus         string
	memory       string
	dryRun       bool
)

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
	runToolCmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the docker command without running it")
	runToolCmd.Flags().SetInterspersed(false)
}

var runToolCmd = &cobra.Command{
	Use:       "run [flags] <tool> [args...]",
	Short:     "Run a tool container",
	Long:      `Run a tool container in the current directory.`,
	Args:      cobra.ArbitraryArgs,
	ValidArgs: tools.Names(),
	RunE:      runTool,
	Hidden:    false,
}

func runTool(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	toolName := args[0]
	imageName, err := tools.ImageName(toolName)
	if err != nil {
		return err
	}

	toolArgs := args[1:]

	skipEntrypoint := len(toolArgs) > 0 && toolArgs[0] == "--"
	if skipEntrypoint {
		toolArgs = toolArgs[1:]
	}

	cwd, _ := os.Getwd()
	rc := config.FindAndLoad(cwd)

	containerHome := docker.ResolveContainerHome(imageName)

	toolConfig := tools.Configs[toolName]
	if err := toolConfig.Setup(toolHome); err != nil {
		return fmt.Errorf("setup %s: %w", toolName, err)
	}

	home, _ := os.UserHomeDir()
	var volumes []string
	var spec config.RunSpec
	volumes = append(toolConfig.Mounts(home), volumes...)
	spec.TmpfsExecTmp = toolConfig.TmpfsExecTmp
	if env := os.Getenv("AGENTIC_EXTRA_MOUNTS"); env != "" {
		for m := range strings.SplitSeq(env, ",") {
			if m != "" {
				volumes = append(volumes, m)
			}
		}
	}
	volumes = append(volumes, extraVolumes...)
	volumes = append(volumes, rc.ExtraMounts...)

	if pidsLimit == "" {
		pidsLimit = rc.PidsLimit
	}
	if cpus == "" {
		cpus = rc.CPUs
	}
	if memory == "" {
		memory = rc.Memory
	}

	if err := ensureNamedVolumes(volumes, toolHome, containerHome); err != nil {
		return err
	}

	rs := docker.RunSpec{
		Image:          imageName,
		ToolHome:       toolHome,
		ContainerHome:  containerHome,
		Volumes:        volumes,
		SkipEntrypoint: skipEntrypoint,
		Spec:           spec,
		PidsLimit:      pidsLimit,
		CPUs:           cpus,
		Memory:         memory,
		DryRun:         dryRun,
	}

	return runContainer(rs, toolArgs)
}
