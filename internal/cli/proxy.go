package cli

import (
	"fmt"
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/settings"
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

// resolveProxyMode reads the proxy-related flags and resolves them against rc into the effective proxy mode.
func resolveProxyMode(cmd *cobra.Command, rc *config.AgenticRC) (enabled, monitor bool) {
	noProxy, _ := cmd.Flags().GetBool("no-proxy")
	monitorFlag, _ := cmd.Flags().GetBool("proxy-monitor")
	proxyFlag, _ := cmd.Flags().GetBool("proxy")

	return settings.ProxyMode(settings.ProxyInput{NoProxy: noProxy, MonitorFlag: monitorFlag, ProxyFlag: proxyFlag}, rc)
}

// ensureProxyImage builds the proxy image if it is not already present or if
// its CLI version label does not match the running CLI version, so `--proxy`
// automatically picks up proxy changes shipped with a CLI update.
func ensureProxyImage(cmd *cobra.Command) error {
	info, err := inspectImage(tools.ProxyImage)
	if err != nil {
		return err
	}
	if info != nil && info.CLIVersion == buildinfo.Version {
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
