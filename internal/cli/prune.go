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
		"The build cache can't be limited to agentic's builds, so it includes your other builds too.",
	Args: cobra.NoArgs,
	RunE: runPrune,
}

func init() {
	rootCmd.AddCommand(pruneCmd)

	pruneCmd.Flags().Bool("force", false, "clear the build cache without asking")
}

func runPrune(cmd *cobra.Command, _ []string) error {
	force, _ := cmd.Flags().GetBool("force")
	confirmCache := func() bool {
		return force || newTTYPrompter().ConfirmCachePrune()
	}

	logging.Infof("pruning unused resources")
	return clean.New(toolHome, dockerClient).Prune(confirmCache)
}
