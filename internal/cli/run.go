package cli

import (
	"fmt"
	"os"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/mount"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/run"
	"github.com/spf13/cobra"
)

var (
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

func init() {
	rootCmd.AddCommand(runToolCmd)

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

	req := runRequest(cmd, inv)
	rs, cleanup, err := run.New(dockerClient).Prepare(req, ttyPrompter{})
	defer cleanup()
	if err != nil {
		return err
	}

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

// runRequest collects the run flags for inv; Prepare resolves them against inv's config.
func runRequest(cmd *cobra.Command, inv invocation) run.Request {
	flags := runtimeFlags(cmd)
	flags.TrustDir = trustDir
	flags.DryRun = dryRun
	flags.Registry = collectRegistry(cmd)
	flags.Volumes = extraVolumes
	flags.Secrets = flagSecrets
	flags.ReadOnlyMounts = flagReadOnlyMounts
	flags.Env = flagEnv

	return run.Request{
		Target: run.Target{
			ToolName:       inv.toolName,
			ImageName:      inv.imageName,
			SkipEntrypoint: inv.skipEntrypoint,
		},
		Tool: inv.toolConfig,
		Project: run.Project{
			Dir:    inv.cwd,
			Layers: inv.layers,
			RC:     inv.rc,
		},
		Flags: flags,
	}
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
