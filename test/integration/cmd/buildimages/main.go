//go:build integration

// Command buildimages builds the integration test images once, before the test packages run in parallel.
package main

import (
	"fmt"
	"os"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
)

func main() {
	if err := platform.BuildImages(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
