// Command agentic-proxy is the egress allowlist proxy that runs in the proxy sidecar.
package main

import (
	"os"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
)

func main() {
	iostreams.SetDefault(iostreams.System())

	if err := run(); err != nil {
		logging.Error(err.Error())
		os.Exit(1)
	}
}

// run starts the proxy from its environment, returning why it stopped.
func run() error {
	cfg, err := proxy.ConfigFromEnv()
	if err != nil {
		return err
	}
	return proxy.Run(cfg)
}
