//go:build integration

// Command cleanup removes the fakes every integration test run left behind, also when a test binary died.
package main

import (
	"fmt"
	"os"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
)

func main() {
	if err := platform.RemoveAllFakes(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
