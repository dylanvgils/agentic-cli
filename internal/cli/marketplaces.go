package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/marketplace"
	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/tools"
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
	baseDir := filepath.Join(toolHome, tools.MarketplacesDirName)

	reg, err := loadMarketplaceRegistry(baseDir)
	if err != nil {
		return err
	}

	dirNames, err := marketplaceCloneDirs(baseDir)
	if err != nil {
		return err
	}

	if len(dirNames) == 0 {
		fmt.Println("No synced marketplaces found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "NAME\tURL\tREFERENCED BY\tSTALE"); err != nil {
		return err
	}

	for _, dirName := range dirNames {
		entry, tracked := reg.Marketplaces[dirName]
		if !tracked {
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", dirName, "(untracked)", "(untracked)", "-"); err != nil {
				return err
			}
			continue
		}

		row := fmt.Sprintf("%s\t%s\t%s\t%s\n", entry.Name, entry.URL, formatProjects(entry.Projects), yesNo(entry.Stale))
		if _, err := fmt.Fprint(w, row); err != nil {
			return err
		}
	}

	return w.Flush()
}

func runMarketplacesPrune(_ *cobra.Command, _ []string) error {
	baseDir := filepath.Join(toolHome, tools.MarketplacesDirName)

	reg, err := loadMarketplaceRegistry(baseDir)
	if err != nil {
		return err
	}

	dirNames, err := marketplaceCloneDirs(baseDir)
	if err != nil {
		return err
	}

	updated := &marketplace.Registry{Marketplaces: map[string]marketplace.RegistryEntry{}}
	for _, dirName := range dirNames {
		entry, tracked := reg.Marketplaces[dirName]
		if !tracked {
			output.Stepf("%s: no usage record, skipping (run `agentic run` from a project that uses it, or remove manually)", dirName)
			continue
		}

		live := liveMarketplaceProjects(dirName, entry.Projects)
		if len(live) == 0 {
			if err := os.RemoveAll(filepath.Join(baseDir, dirName)); err != nil {
				return err
			}
			output.Stepf("removed: %s (no project references it)", entry.Name)
			continue
		}

		entry.Projects = live
		updated.Marketplaces[dirName] = entry
		output.Stepf("kept: %s (used by %s)", entry.Name, strings.Join(live, ", "))
	}

	return saveMarketplaceRegistry(baseDir, updated)
}

// liveMarketplaceProjects re-checks each project's current config, dropping
// any that no longer declare a marketplace matching dirName.
func liveMarketplaceProjects(dirName string, projects []string) []string {
	var live []string
	for _, dir := range projects {
		rc, err := config.FindAndLoad(dir)
		if err != nil {
			continue
		}
		for _, m := range rc.Marketplaces {
			if marketplace.CloneDirName(m.Name, m.URL) == dirName {
				live = append(live, dir)
				break
			}
		}
	}
	return live
}

// marketplaceCloneDirs lists baseDir's subdirectories, sorted. Missing baseDir is not an error.
func marketplaceCloneDirs(baseDir string) ([]string, error) {
	entries, err := os.ReadDir(baseDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	return names, nil
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
