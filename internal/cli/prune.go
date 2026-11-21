package cli

import (
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/usecase/clean"
	"github.com/spf13/cobra"
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Free disk space without removing tool images",
	Long: "Remove leftover sidecars and run networks, dangling agentic images and, after confirmation, Docker's unused build cache. " +
		"With --logs, also remove every run's logs regardless of age. " +
		"The build cache can't be limited to agentic's builds, so it includes your other builds too.",
	Args: cobra.NoArgs,
	RunE: runPrune,
}

func init() {
	rootCmd.AddCommand(pruneCmd)

	pruneCmd.Flags().Bool("force", false, "clear the build cache without asking")
	pruneCmd.Flags().Bool("logs", false, "also remove every run's logs, regardless of age")
}

func runPrune(cmd *cobra.Command, _ []string) error {
	force, _ := cmd.Flags().GetBool("force")
	logs, _ := cmd.Flags().GetBool("logs")
	confirmCache := func() bool {
		return force || newTTYPrompter().ConfirmCachePrune()
	}

	logging.Info("pruning unused resources")
	return clean.New(toolHome, dockerClient).Prune(logs, confirmCache)
}
