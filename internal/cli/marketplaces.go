package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/spf13/cobra"
)

var marketplacesCmd = &cobra.Command{
	Use:   "marketplaces",
	Short: "Manage synced marketplace clones",
	Long:  "Manage marketplace clones downloaded by `agentic run` under $AGENTIC_HOME/marketplaces.",
}

var marketplacesListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List synced marketplace clones and the projects referencing them",
	Args:    cobra.NoArgs,
	RunE:    runMarketplacesList,
}

var marketplacesPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove marketplace clones no longer referenced by any known project",
	Args:  cobra.NoArgs,
	RunE:  runMarketplacesPrune,
}

func init() {
	rootCmd.AddCommand(marketplacesCmd)
	marketplacesCmd.AddCommand(marketplacesListCmd, marketplacesPruneCmd)

	defaultHome := platform.ToolHomeDefault()
	if env := os.Getenv("AGENTIC_HOME"); env != "" {
		defaultHome = env
	}

	marketplacesCmd.PersistentFlags().StringVar(&toolHome, "home", defaultHome,
		"agentic data directory (overrides $AGENTIC_HOME)")
}

func runMarketplacesList(_ *cobra.Command, _ []string) error {
	baseDir := filepath.Join(toolHome, marketplace.MarketplacesDirName)

	registry, err := loadMarketplaceRegistry(baseDir)
	if err != nil {
		return err
	}

	dirNames, err := marketplace.CloneDirs(baseDir)
	if err != nil {
		return err
	}

	if len(dirNames) == 0 {
		fmt.Println("No synced marketplaces found.")
		return nil
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "NAME\tURL\tREFERENCED BY\tSTALE"); err != nil {
		return err
	}

	for _, dirName := range dirNames {
		entries := registry.Marketplaces[dirName]
		if len(entries) == 0 {
			if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", dirName, "(untracked)", "(untracked)", "-"); err != nil {
				return err
			}
			continue
		}

		for _, entry := range entries {
			row := fmt.Sprintf("%s\t%s\t%s\t%s\n", entry.Name, entry.URL, formatProjects(entry.Projects), yesNo(entry.Stale))
			if _, err := fmt.Fprint(writer, row); err != nil {
				return err
			}
		}
	}

	return writer.Flush()
}

func runMarketplacesPrune(_ *cobra.Command, _ []string) error {
	baseDir := filepath.Join(toolHome, marketplace.MarketplacesDirName)

	reg, err := loadMarketplaceRegistry(baseDir)
	if err != nil {
		return err
	}

	updated, report, err := marketplace.Prune(baseDir, reg)
	if err != nil {
		return err
	}

	for _, a := range report {
		switch a.Kind {
		case marketplace.PruneNoRecord:
			logging.Stepf("%s: no usage record, skipping (run `agentic run` from a project that uses it, or remove manually)", a.DirName)
		case marketplace.PruneRemoved:
			logging.Stepf("removed: %s (no project references it)", a.Name)
		case marketplace.PruneDropped:
			logging.Stepf("dropped: %s (no project references it; clone dir kept for other name(s))", a.Name)
		case marketplace.PruneKept:
			logging.Stepf("kept: %s (used by %s)", a.Name, strings.Join(a.Projects, ", "))
		}
	}

	return saveMarketplaceRegistry(baseDir, updated)
}

func formatProjects(projects []string) string {
	if len(projects) == 0 {
		return "(untracked)"
	}

	labeled := make([]string, len(projects))
	for i, dir := range projects {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			labeled[i] = dir + " (missing)"
		} else {
			labeled[i] = dir
		}
	}

	return strings.Join(labeled, ", ")
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
