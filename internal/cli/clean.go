package cli

import (
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/usecase/clean"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:               "clean [tool]",
	Short:             "Remove tool image(s)",
	Long:              "Remove tool image(s). Cleans all tools and base images if no tool specified.",
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: builtToolNamesFunc,
	RunE:              runClean,
}

func init() {
	rootCmd.AddCommand(cleanCmd)

	addNamespaceFlag(cleanCmd)
	addAllFlag(cleanCmd)
}

func runClean(cmd *cobra.Command, args []string) error {
	rc, err := config.FindAndLoadFromCwd()
	if err != nil {
		return err
	}

	namespace := resolveNamespace(cmd, rc)
	all, _ := cmd.Flags().GetBool("all")

	var filterTool string
	if len(args) > 0 {
		filterTool = args[0]
	}

	scope := clean.Scope{
		Names:      toolNames(args),
		FilterTool: filterTool,
		Namespace:  namespace,
		All:        all,
	}

	targets, err := clean.Resolve(scope)
	if err != nil {
		return err
	}

	if err := clean.Apply(targets); err != nil {
		return err
	}

	if len(args) == 0 {
		return clean.GlobalResources(toolHome)
	}

	return nil
}
