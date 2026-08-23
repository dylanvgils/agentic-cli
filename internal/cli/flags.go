package cli

import (
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
	"github.com/spf13/cobra"
)

// addNamespaceFlag registers the --namespace flag on the given command.
func addNamespaceFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("namespace", "n", "", "image namespace (overrides .agenticrc.toml namespace)")
	_ = cmd.RegisterFlagCompletionFunc("namespace", namespacesFunc)
}

// addRegistryFlag registers the --registry flag on the given command.
func addRegistryFlag(cmd *cobra.Command) {
	cmd.Flags().String("registry", "", "registry prefix for base images (e.g. myregistry.example.com); overrides agentic.json registry")
}

// addAllFlag registers the --all flag on the given command.
func addAllFlag(cmd *cobra.Command) {
	cmd.Flags().BoolP("all", "a", false, "operate on all namespaces, not just the active one")
}

// addVersionFlags registers a --<layer> version flag for every known layer on the given command.
func addVersionFlags(cmd *cobra.Command) {
	for _, name := range tools.KnownLayers() {
		cmd.Flags().String(name, "", tools.LayerFlagDesc[name]+" version (default: "+tools.DefaultVersions.ForLayer(name)+")")
	}
}

// resolveNamespace returns the effective namespace, preferring the --namespace flag over the rc file value.
func resolveNamespace(cmd *cobra.Command, rc *config.AgenticRC) string {
	v, _ := cmd.Flags().GetString("namespace")
	return resolve.Namespace(v, rc)
}

// collectRegistry returns the registry prefix from the --registry flag or the tool home config.
func collectRegistry(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("registry")
	return resolve.Registry(v, toolHome)
}

// addBuildFlags registers the version and dry-run flags shared by the build and
// update commands. --no-cache is registered separately because its description
// differs between the two commands.
func addBuildFlags(cmd *cobra.Command) {
	cmd.Flags().StringSlice("base", nil, "extra runtime(s) to layer on top of debian; repeatable or comma-separated (e.g. --base node --base java or --base node,java)")
	cmd.Flags().StringSlice("base-exact", nil, "extra runtime(s) to layer on top of debian, replacing .agenticrc.toml's bases entirely instead of merging with it; repeatable or comma-separated; pass --base-exact= for debian only")
	cmd.Flags().StringSlice("apt", nil, "apt packages to install in the base stage; repeatable or comma-separated (e.g. --apt make --apt gcc or --apt make,gcc)")
	cmd.Flags().StringSlice("apt-exact", nil, "apt packages to install in the base stage, replacing .agenticrc.toml's apt_packages entirely instead of merging with it; repeatable or comma-separated; pass --apt-exact= for none")
	cmd.Flags().Bool("dry-run", false, "print generated Dockerfile without building")

	cmd.MarkFlagsMutuallyExclusive("base", "base-exact")
	cmd.MarkFlagsMutuallyExclusive("apt", "apt-exact")

	addRegistryFlag(cmd)
	addVersionFlags(cmd)
}

// addResourceLimitFlags registers the --pids-limit, --cpus, and --memory flags shared by the run and instructions commands.
func addResourceLimitFlags(cmd *cobra.Command) {
	cmd.Flags().String("pids-limit", "", "container PID limit")
	cmd.Flags().String("cpus", "", "CPU limit")
	cmd.Flags().String("memory", "", "memory limit")
}

// resolveResourceLimitFlags returns the --pids-limit, --cpus, and --memory flag values.
func resolveResourceLimitFlags(cmd *cobra.Command) (pidsLimit, cpus, memory string) {
	pidsLimit, _ = cmd.Flags().GetString("pids-limit")
	cpus, _ = cmd.Flags().GetString("cpus")
	memory, _ = cmd.Flags().GetString("memory")
	return pidsLimit, cpus, memory
}

// addProxyFlags registers the mutually exclusive --proxy, --no-proxy, and --proxy-monitor flags shared by the run and instructions commands.
func addProxyFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("proxy", false, "route egress through the allowlist proxy (overrides config)")
	cmd.Flags().Bool("no-proxy", false, "disable the egress proxy for this run (overrides config)")
	cmd.Flags().Bool("proxy-monitor", false, "route egress through the proxy without blocking, logging every host (overrides config)")
	cmd.MarkFlagsMutuallyExclusive("proxy", "no-proxy", "proxy-monitor")
}

// buildOptsFromFlags constructs a BuildOptions from the command's flags and the project config.
func buildOptsFromFlags(cmd *cobra.Command, rc *config.AgenticRC) tools.BuildOptions {
	flagBases, _ := cmd.Flags().GetStringSlice("base")
	flagApt, _ := cmd.Flags().GetStringSlice("apt")
	noCache, _ := cmd.Flags().GetBool("no-cache")
	pull, _ := cmd.Flags().GetBool("pull")

	in := resolve.BuildInput{
		Bases:            flagBases,
		BasesExact:       exactFlagValue(cmd, "base-exact"),
		VersionOverrides: collectVersionOverrides(cmd),
		AptPackages:      flagApt,
		AptPackagesExact: exactFlagValue(cmd, "apt-exact"),
		NoCache:          noCache,
		Pull:             pull,
		Registry:         collectRegistry(cmd),
	}
	return resolve.BuildOptions(in, rc)
}

// collectBases merges extra base layers from the project config with those from the --base flag.
func collectBases(cmd *cobra.Command, rc *config.AgenticRC) []string {
	flagBases, _ := cmd.Flags().GetStringSlice("base")
	return resolve.Bases(flagBases, rc)
}

// collectVersions builds the per-layer version map with RC values as defaults, overridden by CLI flags.
func collectVersions(cmd *cobra.Command, rc *config.AgenticRC) map[string]string {
	return resolve.Versions(collectVersionOverrides(cmd), rc)
}

// collectVersionOverrides reads every registered --<layer> flag into a map, omitting unset ones.
func collectVersionOverrides(cmd *cobra.Command) map[string]string {
	overrides := make(map[string]string, len(tools.KnownLayers()))
	for _, name := range tools.KnownLayers() {
		if v, _ := cmd.Flags().GetString(name); v != "" {
			overrides[name] = v
		}
	}
	return overrides
}

// collectAptPackages merges apt packages from the project config with those from the --apt flag.
func collectAptPackages(cmd *cobra.Command, rc *config.AgenticRC) []string {
	flagPkgs, _ := cmd.Flags().GetStringSlice("apt")
	return resolve.AptPackages(flagPkgs, rc)
}

// toolNames returns the single tool name from args, or all known tool names when args is empty.
func toolNames(args []string) []string {
	if len(args) > 0 {
		return []string{args[0]}
	}
	return tools.Names()
}

// exactFlagValue returns a pointer to name's flag value when it was explicitly
// passed, or nil otherwise, so resolve can distinguish "not set" from "set empty".
func exactFlagValue(cmd *cobra.Command, name string) *[]string {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	v, _ := cmd.Flags().GetStringSlice(name)
	return &v
}
