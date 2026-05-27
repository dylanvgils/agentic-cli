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
	flagSecrets  []string
	pidsLimit    string
	cpus         string
	memory       string
	dryRun       bool
	trustDir     bool
)

type parsedArgs struct {
	toolName       string
	imageName      string
	toolArgs       []string
	skipEntrypoint bool
}

type resourceLimits struct {
	pidsLimit string
	cpus      string
	memory    string
}

var runToolCmd = &cobra.Command{
	Use:               "run [flags] <tool> [args...]",
	Short:             "Run a tool container",
	Long:              `Run a tool container in the current directory.`,
	Args:              cobra.ArbitraryArgs,
	ValidArgsFunction: builtToolNamesFunc,
	RunE:              runTool,
	Hidden:            false,
}

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
	runToolCmd.Flags().StringArrayVarP(&flagSecrets, "secret", "s", nil,
		"secret file to mount read-only at /run/secrets/<name> (format: name:/path); repeatable")
	runToolCmd.Flags().StringVar(&pidsLimit, "pids-limit", "", "container PID limit")
	runToolCmd.Flags().StringVar(&cpus, "cpus", "", "CPU limit")
	runToolCmd.Flags().StringVar(&memory, "memory", "", "memory limit")
	runToolCmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the docker command without running it")
	runToolCmd.Flags().BoolVar(&trustDir, "trust-dir", false, "trust the current directory and save it to config")
	runToolCmd.Flags().SetInterspersed(false)
}

func runTool(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	parsedArgs, err := parseArgs(args)
	if err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	rc := config.FindAndLoad(cwd)

	toolConfig := tools.Configs[parsedArgs.toolName]
	if err := toolConfig.Runtime.Setup(toolHome); err != nil {
		return fmt.Errorf("setup %s: %w", parsedArgs.toolName, err)
	}

	if err := checkTrust(cwd, toolHome, trustDir); err != nil {
		return err
	}

	containerHome := docker.ResolveContainerHome(parsedArgs.imageName)
	volumes := collectVolumes(toolConfig.Runtime.Mounts(), extraVolumes, rc)
	secrets := collectSecrets(flagSecrets, rc)
	limits := resolveResourceLimits(pidsLimit, cpus, memory, rc)

	if err := ensureNamedVolumes(volumes, toolHome, containerHome); err != nil {
		return err
	}

	rs := docker.NewRunSpec(parsedArgs.imageName).
		WithToolHome(toolHome).
		WithContainerHome(containerHome).
		WithVolumes(volumes...).
		WithSecrets(secrets...).
		WithSkipEntrypoint(parsedArgs.skipEntrypoint).
		WithTmpfsMounts(toolConfig.Runtime.TmpfsMounts()...).
		WithPidsLimit(limits.pidsLimit).
		WithCPUs(limits.cpus).
		WithMemory(limits.memory).
		WithDryRun(dryRun).
		Build()

	return runContainer(rs, parsedArgs.toolArgs)
}

func parseArgs(args []string) (parsedArgs, error) {
	toolName := args[0]
	imageName, err := tools.ImageName(toolName)
	if err != nil {
		return parsedArgs{}, err
	}

	toolArgs := args[1:]
	skipEntrypoint := len(toolArgs) > 0 && toolArgs[0] == "--"
	if skipEntrypoint {
		toolArgs = toolArgs[1:]
	}

	return parsedArgs{
		toolName:       toolName,
		imageName:      imageName,
		toolArgs:       toolArgs,
		skipEntrypoint: skipEntrypoint,
	}, nil
}

func collectVolumes(toolMounts []string, extra []string, rc *config.AgenticRC) []string {
	volumes := append([]string{}, toolMounts...)

	if env := os.Getenv("AGENTIC_EXTRA_MOUNTS"); env != "" {
		for m := range strings.SplitSeq(env, ",") {
			if m != "" {
				volumes = append(volumes, m)
			}
		}
	}
	volumes = append(volumes, extra...)
	volumes = append(volumes, rc.ExtraMounts...)

	return volumes
}

func collectSecrets(flags []string, rc *config.AgenticRC) []string {
	var secrets []string

	if env := os.Getenv("AGENTIC_SECRETS"); env != "" {
		for s := range strings.SplitSeq(env, ",") {
			if s != "" {
				secrets = append(secrets, s)
			}
		}
	}
	secrets = append(secrets, flags...)
	secrets = append(secrets, rc.Secrets...)

	return secrets
}

func resolveResourceLimits(pidsLimit, cpus, memory string, rc *config.AgenticRC) resourceLimits {
	if pidsLimit == "" {
		pidsLimit = rc.PidsLimit
	}
	if cpus == "" {
		cpus = rc.CPUs
	}
	if memory == "" {
		memory = rc.Memory
	}
	return resourceLimits{pidsLimit: pidsLimit, cpus: cpus, memory: memory}
}
