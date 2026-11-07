//go:build integration

// Command cleanup removes the fakes every test run left behind; --all also removes the test images.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
)

func main() {
	all := flag.Bool("all", false, "also remove the test images and their msb copies")
	flag.Parse()

	remove := platform.RemoveAllFakes
	if *all {
		remove = platform.RemoveTestResources
	}
	if err := remove(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
