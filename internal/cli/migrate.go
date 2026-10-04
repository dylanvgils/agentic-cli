package cli

import (
	"os"

	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Apply any pending migrations to the agentic data directory",
	Long:  "Bring $AGENTIC_HOME's on-disk layout up to date. This also runs automatically before most commands; use this to trigger it explicitly, e.g. to pre-migrate or retry after fixing a failed migration.",
	Args:  cobra.NoArgs,
	RunE:  runMigrate,
}

func init() {
	rootCmd.AddCommand(migrateCmd)

	defaultHome := platform.ToolHomeDefault()
	if env := os.Getenv("AGENTIC_HOME"); env != "" {
		defaultHome = env
	}

	migrateCmd.Flags().StringVar(&toolHome, "home", defaultHome,
		"agentic data directory (overrides $AGENTIC_HOME)")
}

func runMigrate(_ *cobra.Command, _ []string) error {
	applied, err := migrateRun(toolHome)
	if err != nil {
		return err
	}

	if len(applied) == 0 {
		logging.Infof("already up to date")
	}

	return nil
}
