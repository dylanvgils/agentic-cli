package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/platform"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show the effective agentic configuration",
	Long:  `Show the merged configuration from agentic.json and all .agenticrc.toml files.`,
	Args:  cobra.NoArgs,
	RunE:  showConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)

	defaultHome := platform.ToolHomeDefault()
	if env := os.Getenv("AGENTIC_HOME"); env != "" {
		defaultHome = env
	}

	configCmd.Flags().StringVar(&toolHome, "home", defaultHome,
		"agentic data directory (overrides $AGENTIC_HOME)")
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
	layers, err := config.FindLayers(cwd)
	if err != nil {
		return err
	}


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

	if cfg.Registry != "" {
		if _, err := fmt.Fprintf(w, "  registry: %s\n", cfg.Registry); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(w, "  registry: (not set)"); err != nil {
			return err
		}
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
		if _, err := fmt.Fprintln(w, "Project (.agenticrc.toml)"); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w, "  (no .agenticrc.toml files found)")
		return err
	}

	noun := "file"
	if len(layers) > 1 {
		noun = "files"
	}
	if _, err := fmt.Fprintf(w, "Project (.agenticrc.toml, %d %s)\n", len(layers), noun); err != nil {
		return err
	}

	pidsLimit := func(rc *config.AgenticRC) string { return rc.Run.PidsLimit }
	cpus := func(rc *config.AgenticRC) string { return rc.Run.CPUs }
	memory := func(rc *config.AgenticRC) string { return rc.Run.Memory }
	extraMounts := func(rc *config.AgenticRC) []string { return rc.Run.ExtraMounts }
	aptPackages := func(rc *config.AgenticRC) []string { return rc.Build.AptPackages }
	secrets := func(rc *config.AgenticRC) []string { return rc.Run.Secrets }

	if err := printScalarField(w, "namespace", config.EnvNamespace, layers, func(rc *config.AgenticRC) string { return rc.Namespace }, config.DefaultNamespace); err != nil {
		return err
	}
	if err := printBasesField(w, layers); err != nil {
		return err
	}
	if err := printListField(w, "apt_packages", layers, aptPackages); err != nil {
		return err
	}
	if err := printScalarField(w, "pids_limit", config.EnvPidsLimit, layers, pidsLimit, docker.DefaultPidsLimit); err != nil {
		return err
	}
	if err := printScalarField(w, "cpus", config.EnvCPUs, layers, cpus, docker.DefaultCPUs); err != nil {
		return err
	}
	if err := printScalarField(w, "memory", config.EnvMemory, layers, memory, docker.DefaultMemory); err != nil {
		return err
	}
	if err := printListField(w, "extra_mounts", layers, extraMounts); err != nil {
		return err
	}
	return printListField(w, "secrets", layers, secrets)
}

// printScalarField prints a scalar config field. Innermost (last in layers) non-empty RC value
// wins. If no layer sets the field, the env var (if set) is shown with a (ENV_VAR) tag. If
// neither is set and defaultVal is non-empty, the default is shown with a (default) tag.
func printScalarField(w io.Writer, label, envVar string, layers []config.RCLayer, get func(*config.AgenticRC) string, defaultVal string) error {
	for i := len(layers) - 1; i >= 0; i-- {
		if v := get(layers[i].RC); v != "" {
			_, err := fmt.Fprintf(w, "  %s: %s  [%s]\n", label, v, layers[i].Path)
			return err
		}
	}
	if envVar != "" {
		if v := os.Getenv(envVar); v != "" {
			_, err := fmt.Fprintf(w, "  %s: %s  (%s)\n", label, v, envVar)
			return err
		}
	}
	if defaultVal != "" {
		_, err := fmt.Fprintf(w, "  %s: %s  (default)\n", label, defaultVal)
		return err
	}
	_, err := fmt.Fprintf(w, "  %s: (not set)\n", label)
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
		_, err := fmt.Fprintf(w, "  %s: (none)\n", label)
		return err
	}

	if _, err := fmt.Fprintf(w, "  %s:\n", label); err != nil {
		return err
	}

	for _, entry := range entries {
		if _, err := fmt.Fprintf(w, "    - %s  [%s]\n", entry.value, entry.path); err != nil {
			return err
		}
	}

	return nil
}

// printBasesField prints the bases list with versions inlined as basename@version.
func printBasesField(w io.Writer, layers []config.RCLayer) error {
	versions := resolveEffectiveVersions(layers)

	getBases := func(rc *config.AgenticRC) []string {
		result := make([]string, len(rc.Build.Bases))
		for i, name := range rc.Build.Bases {
			if v, ok := versions[name]; ok {
				result[i] = name + "@" + v
			} else {
				result[i] = name
			}
		}
		return result
	}

	return printListField(w, "bases", layers, getBases)
}

// resolveEffectiveVersions builds the version map for bases: innermost RC layer wins, then env vars.
func resolveEffectiveVersions(layers []config.RCLayer) map[string]string {
	versions := map[string]string{}

	for _, layer := range layers {
		for name, v := range layer.RC.Build.Versions {
			if v != "" {
				versions[name] = v
			}
		}
	}

	for _, name := range tools.KnownLayers() {
		if v := os.Getenv(config.EnvVersionVar(name)); v != "" {
			versions[name] = v
		}
	}

	return versions
}
