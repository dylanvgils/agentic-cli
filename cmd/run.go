package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/mount"
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
	proxyFlag    bool
	noProxyFlag  bool
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
		"secret file to mount read-only into the container (format: name:/path[:/container/path]); repeatable")
	runToolCmd.Flags().StringVar(&pidsLimit, "pids-limit", "", "container PID limit")
	runToolCmd.Flags().StringVar(&cpus, "cpus", "", "CPU limit")
	runToolCmd.Flags().StringVar(&memory, "memory", "", "memory limit")
	runToolCmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the docker command without running it")
	runToolCmd.Flags().BoolVar(&trustDir, "trust-dir", false, "trust the current directory and save it to config")
	runToolCmd.Flags().BoolVar(&proxyFlag, "proxy", false, "route egress through the allowlist proxy (overrides config)")
	runToolCmd.Flags().BoolVar(&noProxyFlag, "no-proxy", false, "disable the egress proxy for this run (overrides config)")
	runToolCmd.MarkFlagsMutuallyExclusive("proxy", "no-proxy")
	runToolCmd.Flags().SetInterspersed(false)

	addNamespaceFlag(runToolCmd)
	addRegistryFlag(runToolCmd)
}

func runTool(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	cwd, _ := os.Getwd()
	if mount.IsUNCPath(cwd) {
		return fmt.Errorf("working directory %q is on a network share; Docker cannot bind-mount UNC paths", cwd)
	}

	rc, err := config.FindAndLoad(cwd)
	if err != nil {
		return err
	}

	namespace := resolveNamespace(cmd, rc)

	parsedArgs, err := parseArgs(args, namespace)
	if err != nil {
		return err
	}

	if err := requireImage(parsedArgs.imageName, parsedArgs.toolName); err != nil {
		return err
	}

	toolConfig := tools.Configs[parsedArgs.toolName]
	if err := toolConfig.Runtime.Setup(toolHome); err != nil {
		return fmt.Errorf("setup %s: %w", parsedArgs.toolName, err)
	}

	if err := checkTrust(cwd, toolHome, trustDir); err != nil {
		return err
	}

	proxyEnabled := resolveProxyEnabled(cmd, rc)
	if proxyEnabled && !dryRun {
		if err := ensureProxyImage(cmd, namespace); err != nil {
			return err
		}
	}

	rs, err := buildRunSpec(parsedArgs, toolConfig, rc, collectRegistry(cmd), namespace, proxyEnabled)
	if err != nil {
		return err
	}

	return runContainer(rs, parsedArgs.toolArgs)
}

func parseArgs(args []string, namespace string) (parsedArgs, error) {
	toolName := args[0]
	imageName, err := tools.ImageName(toolName, namespace)
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

func buildRunSpec(args parsedArgs, toolConfig tools.ToolConfig, rc *config.AgenticRC, registry, namespace string, proxyEnabled bool) (docker.RunSpec, error) {
	containerHome := docker.ResolveContainerHome(args.imageName)
	volumes := collectVolumes(toolConfig.Runtime.Mounts(), extraVolumes, rc)
	secrets := collectSecrets(flagSecrets, rc)
	limits := resolveResourceLimits(pidsLimit, cpus, memory, rc)

	if err := ensureNamedVolumes(volumes, toolHome, containerHome, tools.BusyboxImageFor(registry)); err != nil {
		return docker.RunSpec{}, err
	}

	if err := ensureNetwork(); err != nil {
		return docker.RunSpec{}, err
	}

	proxyLogDir, err := proxyLogDir(proxyEnabled)
	if err != nil {
		return docker.RunSpec{}, err
	}

	rs := docker.NewRunSpec(args.imageName).
		WithToolHome(toolHome).
		WithContainerHome(containerHome).
		WithVolumes(volumes...).
		WithSecrets(secrets...).
		WithSkipEntrypoint(args.skipEntrypoint).
		WithTmpfsMounts(toolConfig.Runtime.TmpfsMounts()...).
		WithPidsLimit(limits.pidsLimit).
		WithCPUs(limits.cpus).
		WithMemory(limits.memory).
		WithDryRun(dryRun).
		WithProxy(proxyEnabled, tools.ProxyImageName(namespace), proxyAllowList(toolConfig, rc), proxyLogDir).
		Build()

	return rs, nil
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
	volumes = append(volumes, rc.Run.ExtraMounts...)

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
	secrets = append(secrets, rc.Run.Secrets...)

	return secrets
}

func resolveResourceLimits(pidsLimit, cpus, memory string, rc *config.AgenticRC) resourceLimits {
	run := rc.Run
	if pidsLimit == "" {
		pidsLimit = run.PidsLimit
	}
	if cpus == "" {
		cpus = run.CPUs
	}
	if memory == "" {
		memory = run.Memory
	}
	return resourceLimits{pidsLimit: pidsLimit, cpus: cpus, memory: memory}
}

// requireImage returns an error if imageName does not exist locally.
// If the image is missing but the tool has images under other namespaces,
// the error includes a hint to use --namespace.
func requireImage(image, tool string) error {
	info, err := inspectImage(image)
	if err != nil {
		return err
	}
	if info != nil {
		return nil
	}

	images, err := listAllImages(docker.ToolFilter(tool))
	if err != nil {
		return err
	}

	var namespaces []string
	for _, img := range images {
		namespaces = append(namespaces, img.Namespace)
	}

	if len(namespaces) == 0 {
		return fmt.Errorf("image %q not found; run \"agentic build %s\" to build it", image, tool)
	}

	noun := "namespace"
	if len(namespaces) > 1 {
		noun = "namespaces"
	}
	return fmt.Errorf("image %q not found; %q is available under %s %s - use --namespace or run \"agentic build %s\"",
		image, tool, noun, strings.Join(namespaces, ", "), tool)
}
