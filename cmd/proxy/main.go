package main

import (
	"fmt"
	"os"

	"github.com/dylanvgils/agentic-cli/internal/proxy"
)

func main() {
	if err := proxy.Run(proxy.ConfigFromEnv()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
