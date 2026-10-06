package cli

import (
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/platform"
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

	defaultHome := platform.ToolHomeDefault()
	if env := os.Getenv("AGENTIC_HOME"); env != "" {
		defaultHome = env
	}

	cleanCmd.Flags().StringVar(&toolHome, "home", defaultHome,
		"agentic data directory (overrides $AGENTIC_HOME)")

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

	svc := clean.New(dockerClient)
	targets, err := svc.Resolve(scope)
	if err != nil {
		return err
	}

	if len(targets) > 0 {
		labels := make([]string, len(targets))
		for i, t := range targets {
			labels[i] = t.Label
		}
		logging.Infof("removing %d image(s): %s", len(labels), strings.Join(labels, ", "))
	}

	if err := svc.Apply(targets); err != nil {
		return err
	}

	if len(args) == 0 {
		return svc.GlobalResources(toolHome)
	}

	return nil
}
