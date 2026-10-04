package cli

import (
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
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

	if opts.SkipInstallChecksum {
		logging.Warnf("--skip-install-checksum disables install script integrity verification")
	}

	var tool string
	if len(args) > 0 {
		tool = args[0]
	}

	// RC config bases/apt must not prevent per-image label recovery; only an explicit --base/--base-exact or --apt/--apt-exact flag overrides what the image was built with.
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

	targets, skipped, err := update.Resolve(scope, opts, pullExplicit)
	if err != nil {
		return err
	}

	if len(targets) == 0 {
		if all {
			logging.Infof("no agentic images found; run 'agentic build' first")
		} else if len(args) == 0 {
			logging.Infof("no tools are built; run 'agentic build' first")
		}
		return nil
	}

	images := make([]string, len(targets))
	for i, t := range targets {
		images[i] = t.Image
	}
	logging.Infof("updating %d image(s): %s", len(images), strings.Join(images, ", "))
	for _, image := range skipped {
		logging.Stepf("%s (skipped - not built)", image)
	}

	for _, t := range targets {
		if err := update.Apply(t.Name, t.Image, t.Opts); err != nil {
			return err
		}
	}

	pruneResources()
	return nil
}
