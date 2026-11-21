package cli

import (
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/sandbox"
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

	addRegistryFlag(proxyBuildCmd)
	addRegistryFlag(proxyUpdateCmd)
	addDryRunFlag(proxyBuildCmd, "print the generated Dockerfile instead of building")
	addDryRunFlag(proxyUpdateCmd, "print the generated Dockerfile instead of building")
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

	if dryRun, _ := cmd.Flags().GetBool(dryRunFlagName); dryRun {
		printProxyDockerfile(opts.Registry)
		return nil
	}

	logging.Infof("building %s", tools.ProxyImage)
	return sidecar.New(dockerClient).BuildProxy(opts)
}

func runProxyClean(_ *cobra.Command, _ []string) error {
	logging.Infof("removing %s", tools.ProxyImage)
	return dockerClient.CleanImage(tools.ProxyImage)
}

// printProxyDockerfile prints the proxy image's generated Dockerfile for --dry-run.
func printProxyDockerfile(registry string) {
	content := tools.GenerateProxyDockerfile(buildinfo.Version, registry)
	sandbox.PrintDryRunText(tools.ProxyImage+" Dockerfile", content)
}
