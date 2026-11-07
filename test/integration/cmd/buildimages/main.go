//go:build integration

// Command buildimages builds the integration test images once, before the test packages run in parallel: the base set,
// or with --all every image.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
)

func main() {
	all := flag.Bool("all", false, "build every tool's image too, for the smoke tests")
	flag.Parse()

	set := platform.BaseImages
	if *all {
		set = platform.AllImages
	}
	if err := platform.BuildImages(set); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
