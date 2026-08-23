package cli

import (
	"fmt"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/tools"
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
	opts := buildOptsFromFlags(cmd, rc)
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	all, _ := cmd.Flags().GetBool("all")
	pullExplicit := cmd.Flags().Changed("pull")

	var tool string
	if len(args) > 0 {
		tool = args[0]
	}

	// For update, RC config bases/apt must not prevent per-image label recovery.
	// Only an explicit --base/--base-exact (or --apt/--apt-exact) flag should
	// override what the image was built with. -exact sets opts.BaseExact/AptExact
	// (via resolve.BuildOptions), which makes the recovery layer skip entirely,
	// even for an explicit empty list.
	if !cmd.Flags().Changed("base") && !cmd.Flags().Changed("base-exact") {
		opts.BaseOverride = nil
	}
	if !cmd.Flags().Changed("apt") && !cmd.Flags().Changed("apt-exact") {
		opts.AptPackages = nil
	}

	if dryRun {
		return update.DryRun(tool, namespace, opts)
	}

	// Generate the cache-bust value once so multiple targets for the same tool
	// (e.g. --all updating it across namespaces) can still share cached layers.
	opts.CacheBust = docker.NewCacheBust()

	scope := update.Scope{
		Names:      toolNames(args),
		HasArgs:    len(args) > 0,
		FilterTool: tool,
		Namespace:  namespace,
		All:        all,
	}

	targets, err := update.Resolve(scope, opts, pullExplicit)
	if err != nil {
		return err
	}

	if len(targets) == 0 {
		if all {
			fmt.Println("No agentic images found. Run 'agentic build' first.")
		} else if len(args) == 0 {
			fmt.Println("No tools are built. Run 'agentic build' first.")
		}
		return nil
	}

	for _, t := range targets {
		if err := update.Apply(t.Name, t.Image, t.Opts); err != nil {
			return err
		}
	}

	pruneResources()
	return nil
}
