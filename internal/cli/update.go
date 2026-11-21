package cli

import (
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/usecase/precedence"
	"github.com/dylanvgils/agentic-cli/internal/usecase/update"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update <tool|--all>",
	Short: "Update tool image(s) to latest version",
	Long: "Update tool image(s) to latest version. Checks the latest version available\n" +
		"upstream first and rebuilds the tool step without cache if it's newer, so the\n" +
		"installer fetches the latest version. Also pulls fresh base images, at most\n" +
		"once every 24h per image (--pull=false to skip, --pull to force a check now).\n" +
		"--all updates every agentic image in every namespace.",
	Example: `  agentic update claude
  agentic update --all
  agentic update claude --all
  agentic update claude --base java
  agentic update claude --base java,dotnet
  agentic update claude --base-exact node
  agentic update claude --apt-exact make,gcc
  agentic update claude --no-cache
  agentic update claude --pull=false`,
	Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs, requireArgOrAll("tool")),
	ValidArgs: tools.Names(),
	RunE:      runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().Bool("no-cache", false, "also rebuild base layers (fully fresh build)")
	updateCmd.Flags().Bool("pull", true, "pull the latest base images, at most once every 24h per image (--pull=false to skip, e.g. offline; --pull to force a check now)")

	addBuildFlags(updateCmd)
	addDryRunFlag(updateCmd, "print the generated Dockerfile instead of building")
	addNamespaceFlag(updateCmd)
	addAllFlag(updateCmd, "update every image in every namespace, or the tool's image in every namespace")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	project, err := currentProject(cmd)
	if err != nil {
		return err
	}

	namespace := resolveNamespace(cmd, project.RC)
	opts := updateOptsFromFlags(cmd, project.RC)
	warnSkipInstallChecksum(opts)

	all, _ := cmd.Flags().GetBool(allFlagName)
	scope := update.Scope{
		Tool:      firstArg(args),
		Namespace: namespace,
		All:       all,
	}

	svc := update.New(dockerClient, upstreamClient, toolHome)
	plan, err := svc.Resolve(scope, opts, cmd.Flags().Changed("pull"))
	if err != nil {
		return err
	}

	// Only --all can match nothing; a named tool always resolves to its image
	if len(plan.Targets) == 0 {
		logging.Info("no agentic images found; run 'agentic build' first")
		return nil
	}

	if dryRun, _ := cmd.Flags().GetBool(dryRunFlagName); dryRun {
		return svc.DryRun(plan)
	}

	logUpdateSummary(plan)
	return svc.Apply(plan)
}

// updateOptsFromFlags returns the build options for an update; see precedence.UpdateOptions for why config bases/apt need an explicit flag.
func updateOptsFromFlags(cmd *cobra.Command, rc *config.AgenticRC) tools.BuildOptions {
	baseSet := cmd.Flags().Changed("base") || cmd.Flags().Changed(baseExactFlagName)
	aptSet := cmd.Flags().Changed("apt") || cmd.Flags().Changed(aptExactFlagName)

	return precedence.UpdateOptions(buildInput(cmd), baseSet, aptSet, rc)
}

// logUpdateSummary prints the summary line of images to update.
func logUpdateSummary(plan update.Plan) {
	images := make([]string, len(plan.Targets))
	for i, t := range plan.Targets {
		images[i] = t.Image
	}

	logging.Infof("updating %d image(s): %s", len(images), strings.Join(images, ", "))
}
