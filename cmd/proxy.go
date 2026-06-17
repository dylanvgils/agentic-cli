package cmd

import (
	"github.com/dylanvgils/agentic-cli/internal/proxy"
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
