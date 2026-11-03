// Command agentic-proxy is the egress allowlist proxy that runs in the proxy sidecar.
package main

import (
	"log/slog"
	"os"

	"github.com/dylanvgils/agentic-cli/internal/infra/iostreams"
	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
)

func main() {
	iostreams.SetDefault(iostreams.System())

	// A bad config still gets the logger, with UTC times
	cfg, err := proxy.ConfigFromEnv()
	slog.SetDefault(slog.New(proxy.NewLogHandler(iostreams.Default().Out, cfg.Location())))

	if err == nil {
		err = proxy.Run(cfg)
	}
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
