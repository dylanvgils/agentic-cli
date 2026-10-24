package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/image/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/spf13/cobra"
)

var marketplaceCmd = &cobra.Command{
	Use:   "marketplace",
	Short: "Manage synced marketplace clones",
	Long:  "Manage marketplace clones downloaded by `agentic run` under $AGENTIC_HOME/marketplaces.",
}

var marketplaceListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List synced marketplace clones and the projects referencing them",
	Args:    cobra.NoArgs,
	RunE:    runMarketplaceList,
}

var marketplacePruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove marketplace clones no longer referenced by any known project",
	Args:  cobra.NoArgs,
	RunE:  runMarketplacePrune,
}

var marketplacesCmd = &cobra.Command{
	Use:   "marketplaces",
	Short: "List synced marketplace clones (shortcut for marketplace list)",
	Args:  cobra.NoArgs,
	RunE:  runMarketplaceList,
}

// marketplaceRow is a marketplace clone's row in the list table.
type marketplaceRow struct {
	name         string `table:"NAME"`
	url          string `table:"URL"`
	referencedBy string `table:"REFERENCED BY"`
	stale        string `table:"STALE"`
}

func init() {
	rootCmd.AddCommand(marketplaceCmd, marketplacesCmd)
	marketplaceCmd.AddCommand(marketplaceListCmd, marketplacePruneCmd)
}

func runMarketplaceList(_ *cobra.Command, _ []string) error {
	baseDir := filepath.Join(toolHome, marketplace.MarketplacesDirName)

	registry, err := marketplace.LoadRegistry(baseDir)
	if err != nil {
		return err
	}

	dirNames, err := marketplace.CloneDirs(baseDir)
	if err != nil {
		return err
	}

	if len(dirNames) == 0 {
		logging.Infof("no synced marketplaces found")
		return nil
	}

	return writeTable(logging.Log.Writer(), marketplaceRows(registry, dirNames))
}

func runMarketplacePrune(_ *cobra.Command, _ []string) error {
	baseDir := filepath.Join(toolHome, marketplace.MarketplacesDirName)

	reg, err := marketplace.LoadRegistry(baseDir)
	if err != nil {
		return err
	}

	updated, report, err := marketplace.Prune(baseDir, reg)
	if err != nil {
		return err
	}

	if len(report) > 0 {
		logging.Infof("pruning %d marketplace(s)", len(report))
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

	return marketplace.SaveRegistry(baseDir, updated)
}

// marketplaceRows returns one tab-separated row per registry entry of each clone dir, or an untracked row for a dir with none.
func marketplaceRows(registry *marketplace.Registry, dirNames []string) []marketplaceRow {
	var rows []marketplaceRow
	for _, dirName := range dirNames {
		entries := registry.Marketplaces[dirName]
		if len(entries) == 0 {
			rows = append(rows, marketplaceRow{
				name:         dirName,
				url:          "(untracked)",
				referencedBy: "(untracked)",
			})
			continue
		}

		for _, entry := range entries {
			rows = append(rows, marketplaceRow{
				name:         entry.Name,
				url:          entry.URL,
				referencedBy: formatProjects(entry.Projects),
				stale:        yesNo(entry.Stale),
			})
		}
	}
	return rows
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
