package main

import (
	"fmt"
	"os"

	"github.com/dylanvgils/agentic-cli/internal/proxy"
)

func main() {
	cfg, err := proxy.ConfigFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := proxy.Run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
