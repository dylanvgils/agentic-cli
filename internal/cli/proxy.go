package cli

import (
	"fmt"
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/resolve"
	"github.com/dylanvgils/agentic-cli/internal/usecase/sidecar"
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

// runProxyBuildOrUpdate builds the proxy image; `update` always forces noCache so a stale cached layer can't mask a proxy source or base-image change.
func runProxyBuildOrUpdate(cmd *cobra.Command, noCache bool) error {
	opts := tools.BuildOptions{NoCache: noCache, Registry: collectRegistry(cmd)}

	if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
		logging.Step(tools.ProxyImage)
		content := tools.GenerateProxyDockerfile(buildinfo.Version, opts.Registry)
		_, err := fmt.Println(content)
		return err
	}

	logging.Infof("building %s", tools.ProxyImage)
	if err := sidecar.New(dockerClient).BuildProxy(opts); err != nil {
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
		logging.Infof("removing proxy logs")
		pruneProxyLogs(filepath.Join(toolHome, config.LogsDirName), 0)
	}

	return nil
}

// cleanProxyImage removes the proxy image for `agentic proxy clean`; `agentic clean`'s global sweep has its own copy in internal/usecase/clean.
func cleanProxyImage() error {
	logging.Infof("removing %s", tools.ProxyImage)
	return dockerClient.CleanImage(tools.ProxyImage)
}

// proxyInput reads the proxy-related flags.
func proxyInput(cmd *cobra.Command) resolve.ProxyInput {
	noProxy, _ := cmd.Flags().GetBool("no-proxy")
	monitorFlag, _ := cmd.Flags().GetBool("proxy-monitor")
	proxyFlag, _ := cmd.Flags().GetBool("proxy")

	return resolve.ProxyInput{NoProxy: noProxy, MonitorFlag: monitorFlag, ProxyFlag: proxyFlag}
}
