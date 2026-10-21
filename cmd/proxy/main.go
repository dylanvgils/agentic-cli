package main

import (
	"fmt"
	"os"

	"github.com/dylanvgils/agentic-cli/internal/network/proxy"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
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
