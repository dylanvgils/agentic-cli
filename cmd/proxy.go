package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	if cmd.Flags().Changed("no-proxy") {
		return false
	}
	if cmd.Flags().Changed("proxy") {
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
	return buildProxyImage(image, buildinfo.Version, proxySourceDir(), tools.BuildOptions{Registry: collectRegistry(cmd)})
}

// proxySourceDir returns the agentic module root for dev builds, so the proxy
// image can be compiled from local source. It returns "" for released builds
// (which install the published module instead) and when no agentic source tree
// is found by walking up from the working directory.
func proxySourceDir() string {
	if !buildinfo.IsDev(buildinfo.Version) {
		return ""
	}
	return findModuleRoot(tools.ProxyModulePath)
}

// findModuleRoot walks up from the working directory looking for the go.mod of
// the given module, returning its directory or "" if not found. It verifies the
// module path so an unrelated project's go.mod is never used as source.
func findModuleRoot(modulePath string) string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil && moduleMatches(data, modulePath) {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// moduleMatches reports whether a go.mod declares the given module path.
func moduleMatches(gomod []byte, modulePath string) bool {
	for line := range strings.SplitSeq(string(gomod), "\n") {
		if after, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(after) == modulePath
		}
	}
	return false
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
