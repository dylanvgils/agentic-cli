package cli

import (
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
	"github.com/dylanvgils/agentic-cli/internal/usecase/run"
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

// addBuildFlags registers the base/apt/dry-run flags shared by build and update; --no-cache is registered separately since its description differs between them.
func addBuildFlags(cmd *cobra.Command) {
	cmd.Flags().StringSlice("base", nil, "extra runtime(s) to layer on top of debian; repeatable or comma-separated (e.g. --base node --base java or --base node,java)")
	cmd.Flags().StringSlice("base-exact", nil, "extra runtime(s) to layer on top of debian, replacing .agenticrc.toml's bases entirely instead of merging with it; repeatable or comma-separated; pass --base-exact= for debian only")
	cmd.Flags().StringSlice("apt", nil, "apt packages to install in the base stage; repeatable or comma-separated (e.g. --apt make --apt gcc or --apt make,gcc)")
	cmd.Flags().StringSlice("apt-exact", nil, "apt packages to install in the base stage, replacing .agenticrc.toml's apt_packages entirely instead of merging with it; repeatable or comma-separated; pass --apt-exact= for none")
	cmd.Flags().Bool("dry-run", false, "print generated Dockerfile without building")
	cmd.Flags().Bool("skip-install-checksum", false, "bypass checksum verification of Claude/Copilot/OpenCode install scripts; only use if a build is broken by a stale pinned checksum")

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
func resolveResourceLimitFlags(cmd *cobra.Command) docker.ResourceLimits {
	return limitFlags(cmd, "pids-limit", "cpus", "memory")
}

// addProxyFlags registers the mutually exclusive --proxy, --no-proxy, and --proxy-monitor flags shared by the run and instructions commands.
func addProxyFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("proxy", false, "route egress through the allowlist proxy (overrides config)")
	cmd.Flags().Bool("no-proxy", false, "disable the egress proxy for this run (overrides config)")
	cmd.Flags().Bool("proxy-monitor", false, "route egress through the proxy without blocking, logging every host (overrides config)")
	cmd.MarkFlagsMutuallyExclusive("proxy", "no-proxy", "proxy-monitor")
}

// addDindFlags registers the exclusive --dind/--no-dind pair and the sidecar limit flags.
func addDindFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("dind", false, "start a rootless Docker daemon sidecar the tool can use (overrides config; needs the docker base layer)")
	cmd.Flags().Bool("no-dind", false, "disable the Docker daemon sidecar for this run (overrides config)")
	cmd.Flags().String("dind-pids-limit", "", "Docker sidecar PID limit (defaults to the tool's)")
	cmd.Flags().String("dind-cpus", "", "Docker sidecar CPU limit (defaults to the tool's)")
	cmd.Flags().String("dind-memory", "", "Docker sidecar memory limit (defaults to the tool's)")
	cmd.MarkFlagsMutuallyExclusive("dind", "no-dind")
}

// resolveDindResourceLimitFlags returns the --dind-* limit flag values.
func resolveDindResourceLimitFlags(cmd *cobra.Command) docker.ResourceLimits {
	return limitFlags(cmd, "dind-pids-limit", "dind-cpus", "dind-memory")
}

// proxyInput reads the proxy-related flags.
func proxyInput(cmd *cobra.Command) resolve.ProxyInput {
	noProxy, _ := cmd.Flags().GetBool("no-proxy")
	monitorFlag, _ := cmd.Flags().GetBool("proxy-monitor")
	proxyFlag, _ := cmd.Flags().GetBool("proxy")

	return resolve.ProxyInput{NoProxy: noProxy, MonitorFlag: monitorFlag, ProxyFlag: proxyFlag}
}

// dindInput reads the dind flags.
func dindInput(cmd *cobra.Command) resolve.DindInput {
	dindFlag, _ := cmd.Flags().GetBool("dind")
	noDindFlag, _ := cmd.Flags().GetBool("no-dind")

	return resolve.DindInput{DindFlag: dindFlag, NoDindFlag: noDindFlag}
}

// runtimeFlags reads the flags run and instructions share: home, proxy, dind and resource limits.
func runtimeFlags(cmd *cobra.Command) run.Flags {
	return run.Flags{
		ToolHome:   toolHome,
		Proxy:      proxyInput(cmd),
		Dind:       dindInput(cmd),
		Limits:     resolveResourceLimitFlags(cmd),
		DindLimits: resolveDindResourceLimitFlags(cmd),
	}
}

// buildOptsFromFlags constructs a BuildOptions from the command's flags and the project config.
func buildOptsFromFlags(cmd *cobra.Command, rc *config.AgenticRC) tools.BuildOptions {
	return resolve.BuildOptions(buildInput(cmd), rc)
}

// buildInput reads the build flags.
func buildInput(cmd *cobra.Command) resolve.BuildInput {
	flagBases, _ := cmd.Flags().GetStringSlice("base")
	flagApt, _ := cmd.Flags().GetStringSlice("apt")
	noCache, _ := cmd.Flags().GetBool("no-cache")
	pull, _ := cmd.Flags().GetBool("pull")
	skipInstallChecksum, _ := cmd.Flags().GetBool("skip-install-checksum")

	return resolve.BuildInput{
		Bases:               flagBases,
		BasesExact:          exactFlagValue(cmd, "base-exact"),
		VersionOverrides:    collectVersionOverrides(cmd),
		AptPackages:         flagApt,
		AptPackagesExact:    exactFlagValue(cmd, "apt-exact"),
		NoCache:             noCache,
		Pull:                pull,
		Registry:            collectRegistry(cmd),
		SkipInstallChecksum: skipInstallChecksum,
	}
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

// warnSkipInstallChecksum warns that opts turns off install script verification.
func warnSkipInstallChecksum(opts tools.BuildOptions) {
	if opts.SkipInstallChecksum {
		logging.Warnf("--skip-install-checksum disables install script integrity verification")
	}
}

// toolNames returns the single tool name from args, or all known tool names when args is empty.
func toolNames(args []string) []string {
	if len(args) > 0 {
		return []string{args[0]}
	}
	return tools.Names()
}

// firstArg returns args[0], or "" when there are no args.
func firstArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

// exactFlagValue returns name's flag value if explicitly passed, or nil otherwise, so resolve can distinguish "not set" from "set empty".
func exactFlagValue(cmd *cobra.Command, name string) *[]string {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	v, _ := cmd.Flags().GetStringSlice(name)
	return &v
}

// limitFlags reads the named PID, CPU and memory flags into ResourceLimits.
func limitFlags(cmd *cobra.Command, pidsLimitFlag, cpusFlag, memoryFlag string) docker.ResourceLimits {
	pidsLimit, _ := cmd.Flags().GetString(pidsLimitFlag)
	cpus, _ := cmd.Flags().GetString(cpusFlag)
	memory, _ := cmd.Flags().GetString(memoryFlag)

	return docker.ResourceLimits{PidsLimit: pidsLimit, CPUs: cpus, Memory: memory}
}
