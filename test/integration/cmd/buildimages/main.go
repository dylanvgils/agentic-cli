//go:build integration

// Command buildimages builds the images the named test packages need (all without args), before they run.
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
