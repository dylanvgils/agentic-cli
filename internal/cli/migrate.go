package cli

import (
	"fmt"
	"os"

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
		fmt.Println("already up to date")
		return nil
	}

	for _, m := range applied {
		fmt.Printf("applied migration %d: %s\n", m.Version, m.Description)
	}

	return nil
}
