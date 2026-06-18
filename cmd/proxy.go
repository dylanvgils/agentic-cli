package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/proxy"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/spf13/cobra"
)

// proxyCmd runs the egress forward proxy inside the proxy sidecar container. It
// is hidden because users never invoke it directly: agentic starts it for them
// when proxy mode is enabled. Configuration comes from the proxy environment
// variables (see internal/proxy.ConfigFromEnv).
var proxyCmd = &cobra.Command{
	Use:    "__proxy",
	Short:  "Run the egress allowlist proxy (internal)",
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE:   runProxy,
}

func init() {
	rootCmd.AddCommand(proxyCmd)
}

func runProxy(_ *cobra.Command, _ []string) error {
	return proxy.Run(proxy.ConfigFromEnv())
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

// ensureProxyImage builds the proxy image for the namespace if it is not
// already present, so `--proxy` works without a separate build step.
func ensureProxyImage(cmd *cobra.Command, namespace string) error {
	image := tools.ProxyImageName(namespace)

	info, err := inspectImage(image)
	if err != nil {
		return err
	}
	if info != nil {
		return nil
	}

	output.Step("building proxy image " + image)
	return buildProxyImage(image, buildinfo.Version, buildinfo.DevSourceDir(tools.ProxyModulePath), tools.BuildOptions{Registry: collectRegistry(cmd)})
}

// proxyLogDir returns the host directory for proxy access logs, creating it when
// the proxy is enabled. Returns an empty string when the proxy is off.
func proxyLogDir(proxyEnabled bool) (string, error) {
	if !proxyEnabled {
		return "", nil
	}

	dir := filepath.Join(toolHome, "proxy")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create proxy log dir: %w", err)
	}
	return dir, nil
}
