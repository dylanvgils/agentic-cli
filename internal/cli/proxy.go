package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/housekeeping"
	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)

// proxyCmd groups commands that manage the egress proxy sidecar image.
var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Manage the egress proxy image",
}

var proxyBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the proxy image",
	Args:  cobra.NoArgs,
	RunE:  runProxyBuild,
}

var proxyUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Force a fresh proxy image build",
	Args:  cobra.NoArgs,
	RunE:  runProxyUpdate,
}

var proxyCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove the proxy image",
	Args:  cobra.NoArgs,
	RunE:  runProxyClean,
}

func init() {
	rootCmd.AddCommand(proxyCmd)
	proxyCmd.AddCommand(proxyBuildCmd, proxyUpdateCmd, proxyCleanCmd)

	proxyBuildCmd.Flags().Bool("no-cache", false, "disable Docker layer cache for a fully fresh build")
	proxyBuildCmd.Flags().Bool("dry-run", false, "print the generated Dockerfile instead of building")

	proxyUpdateCmd.Flags().Bool("dry-run", false, "print the generated Dockerfile instead of building")

	proxyCleanCmd.Flags().Bool("logs", false, "also remove all proxy access logs, regardless of age")

	addRegistryFlag(proxyBuildCmd)
	addRegistryFlag(proxyUpdateCmd)
}

func runProxyBuild(cmd *cobra.Command, _ []string) error {
	noCache, _ := cmd.Flags().GetBool("no-cache")
	return runProxyBuildOrUpdate(cmd, noCache)
}

func runProxyUpdate(cmd *cobra.Command, _ []string) error {
	return runProxyBuildOrUpdate(cmd, true)
}

// runProxyBuildOrUpdate builds the proxy image, forcing a cache-free rebuild
// when noCache is true. `build` only forces it via --no-cache; `update`
// always forces it - that's the mechanism for picking up a proxy source or
// base-image change that an existing cached image would otherwise mask.
func runProxyBuildOrUpdate(cmd *cobra.Command, noCache bool) error {
	opts := tools.BuildOptions{NoCache: noCache, Registry: collectRegistry(cmd)}

	if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
		output.Step(tools.ProxyImage)
		content := tools.GenerateProxyDockerfile(buildinfo.Version, opts.Registry)
		_, err := fmt.Println(content)
		return err
	}

	if err := buildProxyImageNow(opts); err != nil {
		return err
	}

	pruneResources()
	return nil
}

func runProxyClean(cmd *cobra.Command, _ []string) error {
	if err := cleanProxyImage(); err != nil {
		return err
	}

	if logs, _ := cmd.Flags().GetBool("logs"); logs {
		output.Step("proxy logs")
		pruneProxyLogs(filepath.Join(toolHome, "proxy"), 0)
	}

	return nil
}

// cleanProxyImage removes the proxy image. Shared by `agentic proxy clean`
// and the no-arg `agentic clean`'s global resource sweep.
func cleanProxyImage() error {
	output.Step(tools.ProxyImage)
	return cleanImage(tools.ProxyImage)
}

// resolveProxyEnabled determines whether the egress proxy is on for this run.
// An explicit flag wins over config; --no-proxy beats --proxy; otherwise the
// config value applies, defaulting to off.
func resolveProxyEnabled(cmd *cobra.Command, rc *config.AgenticRC) bool {
	noProxy, _ := cmd.Flags().GetBool("no-proxy")
	if noProxy {
		return false
	}

	proxy, _ := cmd.Flags().GetBool("proxy")
	if proxy {
		return true
	}

	if rc.Run.Proxy.Enabled != nil {
		return *rc.Run.Proxy.Enabled
	}
	return false
}

// proxyAllowList merges the tool's baseline allowlist with user-configured hosts.
func proxyAllowList(toolConfig tools.ToolConfig, rc *config.AgenticRC) []string {
	allow := append([]string{}, toolConfig.Runtime.AllowedHosts...)
	return append(allow, rc.Run.Proxy.AllowedHosts...)
}

// ensureProxyImage builds the proxy image if it is not already present, so
// `--proxy` works without a separate build step.
func ensureProxyImage(cmd *cobra.Command) error {
	info, err := inspectImage(tools.ProxyImage)
	if err != nil {
		return err
	}
	if info != nil {
		return nil
	}

	return buildProxyImageNow(tools.BuildOptions{Registry: collectRegistry(cmd)})
}

// buildProxyImageNow builds the proxy image unconditionally - the caller
// decides whether to check for an existing image first.
func buildProxyImageNow(opts tools.BuildOptions) error {
	output.Step(tools.ProxyImage)
	return buildProxyImage(tools.ProxyImage, buildinfo.Version, buildinfo.DevSourceDir(tools.ProxyModulePath), opts)
}

// proxyLogDir returns the host directory for proxy access logs, creating it
// when the proxy is enabled and pruning any logs older than the configured
// retention window. Returns an empty string when the proxy is off.
func proxyLogDir(proxyEnabled bool) (string, error) {
	if !proxyEnabled {
		return "", nil
	}

	dir := filepath.Join(toolHome, "proxy")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create proxy log dir: %w", err)
	}

	housekeeping.PruneProxyLogs(dir, time.Duration(proxyRetentionDays())*24*time.Hour)

	return dir, nil
}

// proxyRetentionDays resolves the proxy log retention window in days from
// agentic.json, falling back to the default when unset. This is a host-level
// housekeeping setting, not a per-project or per-run one, so it does not come
// from .agenticrc.toml or a CLI flag - it's edited the same way as the other
// global settings in agentic.json (e.g. registry).
func proxyRetentionDays() int {
	if cfg, err := config.LoadConfig(toolHome); err == nil && cfg.ProxyLogRetentionDays > 0 {
		return cfg.ProxyLogRetentionDays
	}
	return housekeeping.DefaultProxyLogRetentionDays
}
