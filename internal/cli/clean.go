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

	all, _ := cmd.Flags().GetBool("all")
	scope := clean.Scope{
		Names:      toolNames(args),
		FilterTool: firstArg(args),
		Namespace:  resolveNamespace(cmd, rc),
		All:        all,
	}

	svc := clean.New(dockerClient)
	targets, err := svc.Resolve(scope)
	if err != nil {
		return err
	}

	logCleanSummary(targets)
	if err := svc.Apply(targets); err != nil {
		return err
	}

	if len(args) == 0 {
		return svc.GlobalResources(toolHome)
	}

	return nil
}

// logCleanSummary prints the summary line of images about to be removed, or nothing when there are none.
func logCleanSummary(targets []clean.Target) {
	if len(targets) == 0 {
		return
	}

	labels := make([]string, len(targets))
	for i, t := range targets {
		labels[i] = t.Label
	}
	logging.Infof("removing %d image(s): %s", len(labels), strings.Join(labels, ", "))
}
