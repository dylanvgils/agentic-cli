package cli

import (
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/build"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build [tool]",
	Short: "Build tool image(s)",
	Long:  "Build tool image(s). Builds all tools if no tool specified.",
	Example: `  agentic build
  agentic build claude
  agentic build claude --base node
  agentic build claude --base node,java
  agentic build claude --base node --node 22
  agentic build claude --base java --java 17
  agentic build claude --base-exact node
  agentic build claude --apt-exact make,gcc`,
	Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
	ValidArgs: tools.Names(),
	RunE:      runBuild,
}

func init() {
	rootCmd.AddCommand(buildCmd)

	buildCmd.Flags().Bool("no-cache", false, "disable Docker layer cache for a fully fresh build")
	buildCmd.Flags().Bool("pull", false, "pull the latest base images from the registry before building")

	addBuildFlags(buildCmd)
	addNamespaceFlag(buildCmd)
}

func runBuild(cmd *cobra.Command, args []string) error {
	rc, err := config.FindAndLoadFromCwd()
	if err != nil {
		return err
	}

	namespace := resolveNamespace(cmd, rc)
	opts := buildOptsFromFlags(cmd, rc)
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	names := toolNames(args)

	if dryRun {
		return build.DryRun(names, opts)
	}

	if err := build.Apply(names, namespace, opts); err != nil {
		return err
	}

	pruneResources()
	return nil
}
