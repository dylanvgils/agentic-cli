//go:build integration

// Command buildimages builds the integration test images once, before the test packages run in parallel: the images
// the packages named as args need, every package's without args.
package main

import (
	"fmt"
	"os"

	"github.com/dylanvgils/agentic-cli/test/integration/platform"
)

func main() {
	if err := platform.BuildImages(platform.ImagesFor(os.Args[1:])); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
