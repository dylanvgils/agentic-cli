package cli

import (
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
	"github.com/dylanvgils/agentic-cli/internal/usecase/update"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update [tool]",
	Short: "Update tool image(s) to latest version",
	Long: "Update tool image(s) to latest version. Checks the latest version available\n" +
		"upstream first and rebuilds the tool step without cache if it's newer, so the\n" +
		"installer fetches the latest version. Also pulls fresh base images, at most\n" +
		"once every 24h per image (--pull=false to skip, --pull to force a check now).\n" +
		"Skips unbuilt tools when no tool specified.",
	Example: `  agentic update
  agentic update claude
  agentic update claude --base java
  agentic update claude --base java,dotnet
  agentic update claude --base-exact node
  agentic update claude --apt-exact make,gcc
  agentic update claude --no-cache
  agentic update claude --pull=false`,
	Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
	ValidArgs: tools.Names(),
	RunE:      runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().Bool("no-cache", false, "also rebuild base layers (fully fresh build)")
	updateCmd.Flags().Bool("pull", true, "pull the latest base images, at most once every 24h per image (--pull=false to skip, e.g. offline; --pull to force a check now)")

	addBuildFlags(updateCmd)
	addNamespaceFlag(updateCmd)
	addAllFlag(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	rc, err := config.FindAndLoadFromCwd()
	if err != nil {
		return err
	}

	namespace := resolveNamespace(cmd, rc)
	opts := updateOptsFromFlags(cmd, rc)
	warnSkipInstallChecksum(opts)

	svc := update.New(dockerClient)

	if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
		return svc.DryRun(firstArg(args), namespace, opts)
	}

	all, _ := cmd.Flags().GetBool("all")
	scope := update.Scope{
		Names:      toolNames(args),
		HasArgs:    len(args) > 0,
		FilterTool: firstArg(args),
		Namespace:  namespace,
		All:        all,
	}

	targets, skipped, err := svc.Resolve(scope, opts, cmd.Flags().Changed("pull"))
	if err != nil {
		return err
	}

	if len(targets) == 0 {
		logNothingToUpdate(scope)
		return nil
	}

	logUpdateSummary(targets, skipped)
	if err := svc.ApplyAll(targets); err != nil {
		return err
	}

	pruneResources()
	return nil
}

// updateOptsFromFlags returns the build options for an update; see resolve.UpdateOptions for why config bases/apt need an explicit flag.
func updateOptsFromFlags(cmd *cobra.Command, rc *config.AgenticRC) tools.BuildOptions {
	baseSet := cmd.Flags().Changed("base") || cmd.Flags().Changed("base-exact")
	aptSet := cmd.Flags().Changed("apt") || cmd.Flags().Changed("apt-exact")

	return resolve.UpdateOptions(buildInput(cmd), baseSet, aptSet, rc)
}

// logNothingToUpdate explains why scope matched nothing; a named tool that isn't built says nothing.
func logNothingToUpdate(scope update.Scope) {
	switch {
	case scope.All:
		logging.Infof("no agentic images found; run 'agentic build' first")
	case !scope.HasArgs:
		logging.Infof("no tools are built; run 'agentic build' first")
	}
}

// logUpdateSummary prints the summary line of images to update, then each unbuilt image that was skipped.
func logUpdateSummary(targets []update.Target, skipped []string) {
	images := make([]string, len(targets))
	for i, t := range targets {
		images[i] = t.Image
	}

	logging.Infof("updating %d image(s): %s", len(images), strings.Join(images, ", "))
	for _, image := range skipped {
		logging.Stepf("%s (skipped - not built)", image)
	}
}
