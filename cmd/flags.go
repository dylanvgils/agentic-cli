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

func resolveNamespace(cmd *cobra.Command, rc *config.AgenticRC) string {
	v, _ := cmd.Flags().GetString("namespace")
	return config.ResolveNamespace(v, rc)
}

// addNamespaceFlag registers the --namespace flag on the given command.
func addNamespaceFlag(cmd *cobra.Command) {
	cmd.Flags().String("namespace", "", "image namespace (overrides AGENTIC_NAMESPACE and .agenticrc namespace)")
}

// addAllFlag registers the --all flag on the given command.
func addAllFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("all", false, "operate on all namespaces, not just the active one")
}

func collectRegistry(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("registry")
	return config.ResolveRegistry(v, toolHome)
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
	v, _ := cmd.Flags().GetString(flag)
	return config.FlagOrEnv(v, env)
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
		if v := flagOrEnv(cmd, name, config.EnvVersionVar(name)); v != "" {
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
			col, config.EnvVersionVar(name), tools.LayerFlagDesc[name], name))
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
