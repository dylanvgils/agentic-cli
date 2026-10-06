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

type parsedArgs struct {
	toolName       string
	imageName      string
	toolArgs       []string
	skipEntrypoint bool
}

// runRequest is what `agentic run` resolved from the working dir, config and args.
type runRequest struct {
	cwd        string
	layers     []config.RCLayer
	rc         *config.AgenticRC
	args       parsedArgs
	toolConfig tools.ToolConfig
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

// target returns the tool and image the run is for.
func (r runRequest) target() run.Target {
	return run.Target{
		ToolName:       r.args.toolName,
		ImageName:      r.args.imageName,
		SkipEntrypoint: r.args.skipEntrypoint,
	}
}

func runTool(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	req, err := loadRunRequest(cmd, args)
	if err != nil {
		return err
	}

	if err := prepareTool(req); err != nil {
		return err
	}

	if err := approveRun(req); err != nil {
		return err
	}

	svc := run.New(dockerClient)

	input, err := resolveRunInput(cmd, svc, req)
	if err != nil {
		return err
	}

	rs, cleanupInstructions, err := svc.BuildWithInstructions(req.target(), input, req.toolConfig, req.rc)
	if err != nil {
		return err
	}
	defer cleanupInstructions()

	return dockerClient.RunContainer(rs, req.args.toolArgs)
}

// loadRunRequest resolves the working dir, project config and tool/image from args.
func loadRunRequest(cmd *cobra.Command, args []string) (runRequest, error) {
	cwd, _ := os.Getwd()
	if mount.IsUNCPath(cwd) {
		return runRequest{}, fmt.Errorf("working directory %q is on a network share; Docker cannot bind-mount UNC paths", cwd)
	}

	// Load the layers once so the credentials approved later are the ones the run uses
	layers, err := config.FindLayers(cwd)
	if err != nil {
		return runRequest{}, err
	}

	rc, err := config.Merge(layers)
	if err != nil {
		return runRequest{}, err
	}

	parsed, err := parseArgs(args, resolveNamespace(cmd, rc))
	if err != nil {
		return runRequest{}, err
	}

	return runRequest{
		cwd:        cwd,
		layers:     layers,
		rc:         rc,
		args:       parsed,
		toolConfig: tools.Configs[parsed.toolName],
	}, nil
}

// prepareTool makes sure the image exists, offers a due tool update, and creates the tool's host files.
func prepareTool(req runRequest) error {
	if err := requireImage(req.args.imageName, req.args.toolName); err != nil {
		return err
	}

	updater := func(tool, image string) error {
		return update.New(dockerClient).ApplyRecovered(tool, image, req.rc)
	}
	if err := toolupdate.New(dockerClient).Check(toolHome, req.rc, req.args.toolName, req.args.imageName, updater); err != nil {
		return err
	}

	if err := req.toolConfig.Runtime.Setup(toolHome); err != nil {
		return fmt.Errorf("setup %s: %w", req.args.toolName, err)
	}

	return nil
}

// approveRun asks the user to trust the working dir and approve any new or changed proxy credentials.
func approveRun(req runRequest) error {
	if err := checkTrust(req.cwd, toolHome, trustDir); err != nil {
		return err
	}

	return checkCredentials(req.layers, toolHome)
}

// resolveRunInput resolves the proxy, credentials and dind settings, ensures their sidecar images, and collects the run flags.
func resolveRunInput(cmd *cobra.Command, svc *run.Service, req runRequest) (run.Input, error) {
	proxyMode, err := resolveProxyMode(cmd, req.rc)
	if err != nil {
		return run.Input{}, err
	}

	// Credentials force the proxy on (see resolve.ProxyMode), so this is a no-op when it is off
	creds, err := svc.ResolveCredentials(req.layers, toolHome)
	if err != nil {
		return run.Input{}, err
	}
	if len(creds) > 0 && !dryRun {
		logging.Infof("injecting credentials for %s", describeInjection(creds))
	}

	dindEnabled := resolveDindEnabled(cmd, req.rc)
	if dindEnabled {
		if err := svc.RequireDockerLayer(req.args.imageName, req.args.toolName); err != nil {
			return run.Input{}, err
		}
	}

	if err := ensureSidecarImages(cmd, proxyMode.Enabled(), dindEnabled); err != nil {
		return run.Input{}, err
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

// ensureSidecarImages builds the enabled sidecars' images when needed; a dry run builds nothing.
func ensureSidecarImages(cmd *cobra.Command, proxyEnabled, dindEnabled bool) error {
	if dryRun {
		return nil
	}

	if proxyEnabled {
		if err := ensureProxyImage(cmd); err != nil {
			return err
		}
	}

	if dindEnabled {
		return ensureDindImage(cmd)
	}

	return nil
}

// ensureDindImage builds the sidecar image if missing, outdated or stale; a failed refresh only warns so offline runs work.
func ensureDindImage(cmd *cobra.Command) error {
	info, err := dockerClient.InspectImage(tools.DindImage)
	if err != nil {
		return err
	}

	reason := docker.ImageRefreshReason(info, tools.DindImageMaxAge)
	if reason == "" {
		return nil
	}

	logging.Infof("building %s (%s)...", tools.DindImage, reason)
	err = dockerClient.BuildDindImage(tools.DindImage, tools.BuildOptions{Registry: collectRegistry(cmd)})
	if err != nil && info != nil {
		logging.Warnf("could not refresh %s, using the existing image: %v", tools.DindImage, err)
		return nil
	}
	return err
}

// describeInjection lists each credential's hosts and secret source for the per-run notice; never the secret itself.
func describeInjection(creds []credentials.Resolved) string {
	parts := make([]string, 0, len(creds))
	for _, cred := range creds {
		parts = append(parts, fmt.Sprintf("%s (%s)", strings.Join(cred.Hosts(), ", "), cred.Source))
	}
	return strings.Join(parts, "; ")
}
