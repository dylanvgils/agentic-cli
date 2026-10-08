package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/credentials"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/run"
	"github.com/dylanvgils/agentic-cli/internal/usecase/sidecar"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolupdate"
	"github.com/dylanvgils/agentic-cli/internal/usecase/update"
	"github.com/spf13/cobra"
)

var (
	toolHome           string
	extraVolumes       []string
	flagSecrets        []string
	flagReadOnlyMounts []string
	flagEnv            []string
	dryRun             bool
	trustDir           bool
)

var runToolCmd = &cobra.Command{
	Use:               "run [flags] <tool> [args...]",
	Short:             "Run a tool container",
	Long:              `Run a tool container in the current directory.`,
	Args:              cobra.ArbitraryArgs,
	ValidArgsFunction: builtToolNamesFunc,
	RunE:              runTool,
	Hidden:            false,
}

type parsedArgs struct {
	toolName       string
	imageName      string
	toolArgs       []string
	skipEntrypoint bool
}

// invocation is an `agentic run` call resolved against the working dir's config.
type invocation struct {
	parsedArgs
	cwd        string
	layers     []config.RCLayer
	rc         *config.AgenticRC
	toolConfig tools.ToolConfig
}

// target returns the tool and image the run is for.
func (inv invocation) target() run.Target {
	return run.Target{
		ToolName:       inv.toolName,
		ImageName:      inv.imageName,
		SkipEntrypoint: inv.skipEntrypoint,
	}
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
		"additional volume mount (format: host:container[:options]); bind mounts are read-only unless options include rw; repeatable")
	runToolCmd.Flags().StringArrayVarP(&flagSecrets, "secret", "s", nil,
		"secret file to mount read-only into the container (format: name:/path[:/container/path]); repeatable")
	runToolCmd.Flags().StringArrayVar(&flagReadOnlyMounts, "read-only-mount", nil,
		"sub-path to force read-only, applied after other mounts (format: host:container); repeatable")
	runToolCmd.Flags().StringArrayVarP(&flagEnv, "env", "e", nil,
		"environment variable to set in the container (format: KEY=VALUE, or KEY to forward the host value); repeatable")
	runToolCmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the docker command without running it")
	runToolCmd.Flags().BoolVar(&trustDir, "trust-dir", false, "trust the current directory and save it to config")
	runToolCmd.Flags().SetInterspersed(false)

	addResourceLimitFlags(runToolCmd)
	addProxyFlags(runToolCmd)
	addDindFlags(runToolCmd)
	addNamespaceFlag(runToolCmd)
	addRegistryFlag(runToolCmd)
}

func runTool(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	inv, err := parseInvocation(cmd, args)
	if err != nil {
		return err
	}

	if err := requireImage(inv.imageName, inv.toolName); err != nil {
		return err
	}
	if err := checkToolUpdate(inv); err != nil {
		return err
	}
	if err := inv.toolConfig.Runtime.Setup(toolHome); err != nil {
		return fmt.Errorf("setup %s: %w", inv.toolName, err)
	}

	svc := run.New(dockerClient)

	if err := svc.CheckTrust(inv.cwd, toolHome, trustDir, ttyPrompter{}); err != nil {
		return err
	}
	if err := svc.CheckCredentials(inv.layers, toolHome, ttyPrompter{}); err != nil {
		return err
	}

	input, err := resolveRunInput(cmd, svc, inv)
	if err != nil {
		return err
	}
	if !input.DryRun {
		if err := sidecar.New(dockerClient).Ensure(input.ProxyMode.Enabled(), input.DindEnabled, input.Registry); err != nil {
			return err
		}
	}

	rs, cleanupInstructions, err := svc.BuildWithInstructions(inv.target(), input, inv.toolConfig, inv.rc)
	if err != nil {
		return err
	}
	defer cleanupInstructions()

	return dockerClient.RunContainer(rs, inv.toolArgs)
}

// parseInvocation loads the working dir's config layers and parses the tool, image and tool args from args.
func parseInvocation(cmd *cobra.Command, args []string) (invocation, error) {
	cwd, _ := os.Getwd()
	if mount.IsUNCPath(cwd) {
		return invocation{}, fmt.Errorf("working directory %q is on a network share; Docker cannot bind-mount UNC paths", cwd)
	}

	// Load the layers once so the credentials approved later are the ones the run uses
	layers, err := config.FindLayers(cwd)
	if err != nil {
		return invocation{}, err
	}

	rc, err := config.Merge(layers)
	if err != nil {
		return invocation{}, err
	}

	parsed, err := parseArgs(args, resolveNamespace(cmd, rc))
	if err != nil {
		return invocation{}, err
	}

	return invocation{
		parsedArgs: parsed,
		cwd:        cwd,
		layers:     layers,
		rc:         rc,
		toolConfig: tools.Configs[parsed.toolName],
	}, nil
}

// checkToolUpdate offers a due update of the tool and applies it when accepted.
func checkToolUpdate(inv invocation) error {
	updater := func(tool, image string) error {
		return update.New(dockerClient).ApplyRecovered(tool, image, inv.rc)
	}

	return toolupdate.New(dockerClient).Check(toolHome, inv.rc, inv.toolName, inv.imageName, ttyPrompter{}.OfferToolUpdate, updater)
}

// resolveRunInput resolves the proxy, credentials and dind settings and collects the run flags.
func resolveRunInput(cmd *cobra.Command, svc *run.Service, inv invocation) (run.Input, error) {
	proxyMode, err := resolveProxyMode(cmd, inv.rc)
	if err != nil {
		return run.Input{}, err
	}

	// Credentials force the proxy on (see resolve.ProxyMode), so this is a no-op when it is off
	creds, err := svc.ResolveCredentials(inv.layers, toolHome)
	if err != nil {
		return run.Input{}, err
	}
	if len(creds) > 0 && !dryRun {
		logging.Infof("injecting credentials for %s", describeInjection(creds))
	}

	dindEnabled := resolveDindEnabled(cmd, inv.rc)
	if dindEnabled {
		if err := svc.RequireDockerLayer(inv.imageName, inv.toolName); err != nil {
			return run.Input{}, err
		}
	}

	return run.Input{
		ToolHome:       toolHome,
		Volumes:        extraVolumes,
		Secrets:        flagSecrets,
		ReadOnlyMounts: flagReadOnlyMounts,
		Env:            flagEnv,
		Limits:         resolveResourceLimitFlags(cmd),
		DryRun:         dryRun,
		Registry:       collectRegistry(cmd),
		ProxyMode:      proxyMode,
		DindEnabled:    dindEnabled,
		DindLimits:     resolveDindResourceLimitFlags(cmd),
		Credentials:    creds,
	}, nil
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

// requireImage errors if the image doesn't exist locally, hinting at --namespace if the tool has images under other namespaces.
func requireImage(image, tool string) error {
	info, err := dockerClient.InspectImage(image)
	if err != nil {
		return err
	}
	if info != nil {
		return nil
	}

	images, err := dockerClient.ListAllImages(docker.ToolFilter(tool))
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

// describeInjection lists each credential's hosts and secret source for the per-run notice; never the secret itself.
func describeInjection(creds []credentials.Resolved) string {
	parts := make([]string, 0, len(creds))
	for _, cred := range creds {
		parts = append(parts, fmt.Sprintf("%s (%s)", strings.Join(cred.Hosts(), ", "), cred.Source))
	}
	return strings.Join(parts, "; ")
}
