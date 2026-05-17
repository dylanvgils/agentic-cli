package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(configCmd)

	defaultHome := platform.ToolHomeDefault()
	if env := os.Getenv("AGENTIC_HOME"); env != "" {
		defaultHome = env
	}

	configCmd.Flags().StringVar(&toolHome, "home", defaultHome,
		"agentic data directory (overrides $AGENTIC_HOME)")
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show the effective agentic configuration",
	Long:  `Show the merged configuration from agentic.json and all .agenticrc files.`,
	Args:  cobra.NoArgs,
	RunE:  showConfig,
}

func showConfig(cmd *cobra.Command, _ []string) error {
	cliConfig, err := config.LoadConfig(toolHome)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	layers := config.FindLayers(cwd)

	w := cmd.OutOrStdout()
	if err := printGlobalConfig(w, toolHome, cliConfig); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	return printProjectConfig(w, layers)
}

func printGlobalConfig(w io.Writer, home string, cfg *config.CliConfig) error {
	if _, err := fmt.Fprintf(w, "Global (%s)\n", filepath.Join(home, "agentic.json")); err != nil {
		return err
	}

	if len(cfg.TrustedDirs) == 0 {
		_, err := fmt.Fprintln(w, "  trusted_dirs: (none)")
		return err
	}

	if _, err := fmt.Fprintln(w, "  trusted_dirs:"); err != nil {
		return err
	}

	for _, dir := range cfg.TrustedDirs {
		if _, err := fmt.Fprintf(w, "    - %s\n", dir); err != nil {
			return err
		}
	}

	return nil
}

func printProjectConfig(w io.Writer, layers []config.RCLayer) error {
	if len(layers) == 0 {
		if _, err := fmt.Fprintln(w, "Project (.agenticrc)"); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w, "  (no .agenticrc files found)")
		return err
	}

	noun := "file"
	if len(layers) > 1 {
		noun = "files"
	}
	if _, err := fmt.Fprintf(w, "Project (.agenticrc, %d %s)\n", len(layers), noun); err != nil {
		return err
	}

	if err := printScalarField(w, "  pids_limit", layers, func(rc *config.AgenticRC) string { return rc.PidsLimit }); err != nil {
		return err
	}
	if err := printScalarField(w, "  cpus", layers, func(rc *config.AgenticRC) string { return rc.CPUs }); err != nil {
		return err
	}
	if err := printScalarField(w, "  memory", layers, func(rc *config.AgenticRC) string { return rc.Memory }); err != nil {
		return err
	}
	if err := printListField(w, "  extra_mounts", layers, func(rc *config.AgenticRC) []string { return rc.ExtraMounts }); err != nil {
		return err
	}
	return printListField(w, "  secrets", layers, func(rc *config.AgenticRC) []string { return rc.Secrets })
}

// printScalarField prints a scalar config field. Innermost (last in layers) non-empty value wins.
func printScalarField(w io.Writer, label string, layers []config.RCLayer, get func(*config.AgenticRC) string) error {
	for i := len(layers) - 1; i >= 0; i-- {
		if v := get(layers[i].RC); v != "" {
			_, err := fmt.Fprintf(w, "%s: %s  [%s]\n", label, v, layers[i].Path)
			return err
		}
	}
	_, err := fmt.Fprintf(w, "%s: (not set)\n", label)
	return err
}

// printListField prints a list config field, tagging each entry with the layer it came from.
// Entries are shown outermost-first (same order as the effective merge).
func printListField(w io.Writer, label string, layers []config.RCLayer, get func(*config.AgenticRC) []string) error {
	type entry struct {
		value string
		path  string
	}

	var entries []entry
	for _, layer := range layers {
		for _, value := range get(layer.RC) {
			entries = append(entries, entry{value: value, path: layer.Path})
		}
	}

	if len(entries) == 0 {
		_, err := fmt.Fprintf(w, "%s: (none)\n", label)
		return err
	}

	if _, err := fmt.Fprintf(w, "%s:\n", label); err != nil {
		return err
	}

	for _, entry := range entries {
		if _, err := fmt.Fprintf(w, "    - %s  [%s]\n", entry.value, entry.path); err != nil {
			return err
		}
	}

	return nil
}
