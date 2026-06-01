package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)

// resolvePrefix returns the active image prefix.
// Precedence: --prefix flag > AGENTIC_PREFIX env var > .agenticrc prefix field > DefaultPrefix.
func resolvePrefix(cmd *cobra.Command, rc *config.AgenticRC) string {
	if v, _ := cmd.Flags().GetString("prefix"); v != "" {
		return v
	}
	if v := os.Getenv("AGENTIC_PREFIX"); v != "" {
		return v
	}
	if rc != nil && rc.Prefix != "" {
		return rc.Prefix
	}
	return tools.DefaultPrefix
}

// addPrefixFlag registers the --prefix flag on the given command.
func addPrefixFlag(cmd *cobra.Command) {
	cmd.Flags().String("prefix", "", "image name prefix (overrides AGENTIC_PREFIX and .agenticrc prefix)")
}

// addAllFlag registers the --all flag on the given command.
func addAllFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("all", false, "operate on all prefixes, not just the active one")
}

// collectRegistry resolves the registry to use for pulling base images.
// Precedence: --registry flag > agentic.json registry field.
func collectRegistry(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString("registry"); v != "" {
		return v
	}
	if cfg, err := config.LoadConfig(toolHome); err == nil {
		return cfg.Registry
	}
	return ""
}

// addBuildFlags registers the version and dry-run flags shared by the build and
// update commands. --no-cache is registered separately because its description
// differs between the two commands.
func addBuildFlags(cmd *cobra.Command) {
	cmd.Flags().StringSlice("base", nil, "extra runtime(s) to layer on top of node; repeatable or comma-separated (e.g. --base java --base dotnet or --base java,dotnet)")
	cmd.Flags().StringSlice("apt", nil, "apt packages to install in the base stage; repeatable or comma-separated (e.g. --apt make --apt gcc or --apt make,gcc)")
	cmd.Flags().Bool("dry-run", false, "print generated Dockerfile without building")
	cmd.Flags().String("registry", "", "registry prefix for base images (e.g. myregistry.example.com); overrides agentic.json registry")

	addVersionFlags(cmd)
}

func addVersionFlags(cmd *cobra.Command) {
	for _, name := range tools.KnownLayers() {
		cmd.Flags().String(name, "", tools.LayerFlagDesc[name]+" version (default: "+tools.DefaultVersions.ForLayer(name)+")")
	}
}

func flagOrEnv(cmd *cobra.Command, flag, env string) string {
	if v, _ := cmd.Flags().GetString(flag); v != "" {
		return v
	}
	return os.Getenv(env)
}

func buildOptsFromFlags(cmd *cobra.Command) tools.BuildOptions {
	opts := tools.BuildOptions{Versions: map[string]string{}}

	if baseVals, _ := cmd.Flags().GetStringSlice("base"); len(baseVals) > 0 {
		opts.BaseOverride = strings.Join(baseVals, ",")
	} else if v := os.Getenv("AGENTIC_BASE_OVERRIDE"); v != "" {
		opts.BaseOverride = v
	}
	opts.NoCache, _ = cmd.Flags().GetBool("no-cache")
	for _, name := range tools.KnownLayers() {
		if v := flagOrEnv(cmd, name, tools.ExtraEnvVarName(name)); v != "" {
			opts.Versions[name] = v
		}
	}

	opts.AptPackages = collectAptPackages(cmd)
	opts.VerifyApt = len(opts.AptPackages) > 0
	opts.Registry = collectRegistry(cmd)

	return opts
}

func collectAptPackages(cmd *cobra.Command) []string {
	cwd, _ := os.Getwd()
	flagPkgs, _ := cmd.Flags().GetStringSlice("apt")
	return tools.MergePackages(config.AptPackages(cwd), flagPkgs)
}

func toolNames(args []string) []string {
	if len(args) > 0 {
		return []string{args[0]}
	}
	return tools.Names()
}

func extrasEnvDoc() string {
	const col = 24

	lines := []string{"Environment:"}

	for _, name := range tools.KnownLayers() {
		lines = append(lines, fmt.Sprintf("  %-*s %s version (overridden by --%s)",
			col, tools.ExtraEnvVarName(name), tools.LayerFlagDesc[name], name))
	}

	return strings.Join(lines, "\n")
}

func pruneAndReport() error {
	reclaimed, err := pruneImages()
	if err != nil {
		return err
	}

	if reclaimed != "" {
		output.Stepf("pruned dangling images (reclaimed %s)", reclaimed)
	}

	return nil
}
