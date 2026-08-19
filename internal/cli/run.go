package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/run"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolupdate"
	"github.com/dylanvgils/agentic-cli/internal/usecase/update"
	"github.com/spf13/cobra"
)

var (
	toolHome     string
	extraVolumes []string
	flagSecrets  []string
	flagEnv      []string
	dryRun       bool
	trustDir     bool
)

type parsedArgs struct {
	toolName       string
	imageName      string
	toolArgs       []string
	skipEntrypoint bool
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
	runToolCmd.Flags().StringArrayVarP(&flagEnv, "env", "e", nil,
		"environment variable to set in the container (format: KEY=VALUE, or KEY to forward the host value); repeatable")
	runToolCmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the docker command without running it")
	runToolCmd.Flags().BoolVar(&trustDir, "trust-dir", false, "trust the current directory and save it to config")
	runToolCmd.Flags().SetInterspersed(false)

	addResourceLimitFlags(runToolCmd)
	addProxyFlags(runToolCmd)
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

	updater := func(tool, image string) error {
		return update.ApplyRecovered(tool, image, rc)
	}
	if err := toolupdate.Check(toolHome, rc, parsedArgs.toolName, parsedArgs.imageName, updater); err != nil {
		return err
	}

	toolConfig := tools.Configs[parsedArgs.toolName]
	if err := toolConfig.Runtime.Setup(toolHome); err != nil {
		return fmt.Errorf("setup %s: %w", parsedArgs.toolName, err)
	}

	if err := checkTrust(cwd, toolHome, trustDir); err != nil {
		return err
	}

	proxyEnabled, proxyMonitor := resolveProxyMode(cmd, rc)
	if proxyEnabled && !dryRun {
		if err := ensureProxyImage(cmd); err != nil {
			return err
		}
	}

	pidsLimit, cpus, memory := resolveResourceLimitFlags(cmd)

	target := run.Target{
		ToolName:       parsedArgs.toolName,
		ImageName:      parsedArgs.imageName,
		SkipEntrypoint: parsedArgs.skipEntrypoint,
	}
	input := run.Input{
		ToolHome:     toolHome,
		Volumes:      extraVolumes,
		Secrets:      flagSecrets,
		Env:          flagEnv,
		PidsLimit:    pidsLimit,
		CPUs:         cpus,
		Memory:       memory,
		DryRun:       dryRun,
		Registry:     collectRegistry(cmd),
		ProxyEnabled: proxyEnabled,
		ProxyMonitor: proxyMonitor,
	}

	rs, cleanupInstructions, err := run.BuildWithInstructions(target, input, toolConfig, rc)
	if err != nil {
		return err
	}
	defer cleanupInstructions()

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
