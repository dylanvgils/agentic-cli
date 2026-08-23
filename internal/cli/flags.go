package cli

import (
	"maps"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/tools"
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
	return config.ResolveNamespace(v, rc)
}

// collectRegistry returns the registry prefix from the --registry flag or the tool home config.
func collectRegistry(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("registry")
	return config.ResolveRegistry(v, toolHome)
}

// addBuildFlags registers the version and dry-run flags shared by the build and
// update commands. --no-cache is registered separately because its description
// differs between the two commands.
func addBuildFlags(cmd *cobra.Command) {
	cmd.Flags().StringSlice("base", nil, "extra runtime(s) to layer on top of debian; repeatable or comma-separated (e.g. --base node --base java or --base node,java)")
	cmd.Flags().StringSlice("apt", nil, "apt packages to install in the base stage; repeatable or comma-separated (e.g. --apt make --apt gcc or --apt make,gcc)")
	cmd.Flags().Bool("dry-run", false, "print generated Dockerfile without building")

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
	opts := tools.BuildOptions{}

	opts.BaseOverride = collectBases(cmd, rc)
	opts.NoCache, _ = cmd.Flags().GetBool("no-cache")
	opts.Pull, _ = cmd.Flags().GetBool("pull")
	opts.Versions = collectVersions(cmd, rc)
	opts.AptPackages = collectAptPackages(cmd, rc)
	opts.VerifyApt = len(opts.AptPackages) > 0
	opts.Registry = collectRegistry(cmd)
	opts.CustomInstalls = rc.Build.CustomInstalls

	return opts
}

// collectBases merges extra base layers from the project config with those from the --base flag.
func collectBases(cmd *cobra.Command, rc *config.AgenticRC) []string {
	flagBases, _ := cmd.Flags().GetStringSlice("base")
	return tools.SortExtras(tools.MergePackages(rc.Build.Bases, flagBases))
}

// collectVersions builds the per-layer version map with RC values as defaults, overridden by CLI flags.
func collectVersions(cmd *cobra.Command, rc *config.AgenticRC) map[string]string {
	versions := make(map[string]string, len(rc.Build.Versions))
	maps.Copy(versions, rc.Build.Versions)

	for _, name := range tools.KnownLayers() {
		if v, _ := cmd.Flags().GetString(name); v != "" {
			versions[name] = v
		}
	}
	return versions
}

// collectAptPackages merges apt packages from the project config with those from the --apt flag.
func collectAptPackages(cmd *cobra.Command, rc *config.AgenticRC) []string {
	flagPkgs, _ := cmd.Flags().GetStringSlice("apt")
	return tools.MergePackages(rc.Build.AptPackages, flagPkgs)
}

// toolNames returns the single tool name from args, or all known tool names when args is empty.
func toolNames(args []string) []string {
	if len(args) > 0 {
		return []string{args[0]}
	}
	return tools.Names()
}
