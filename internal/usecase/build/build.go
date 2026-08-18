// Package build builds tool images for `agentic build`, printing generated
// Dockerfiles instead of building them in dry-run mode.
package build

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// BuildTool indirects the docker call this package makes, so callers can
// fake it in tests - mirrors the seam convention in internal/cli/root.go.
var BuildTool = docker.BuildTool

// DryRun prints the generated Dockerfile for each tool in names instead of building it.
func DryRun(names []string, opts tools.BuildOptions) error {
	for _, name := range names {
		output.Step(name)
		content, err := tools.GenerateDockerfile(name, opts)
		if err != nil {
			return err
		}
		if _, err := fmt.Println(content); err != nil {
			return err
		}
	}
	return nil
}

// Apply builds each tool image in names under namespace, reporting the
// base/apt overrides in effect for each.
func Apply(names []string, namespace string, opts tools.BuildOptions) error {
	for _, name := range names {
		image, err := tools.ImageName(name, namespace)
		if err != nil {
			return err
		}

		output.Step(image)
		if len(opts.BaseOverride) > 0 {
			output.Detailf("base: %s", strings.Join(opts.BaseOverride, ", "))
		}
		if len(opts.AptPackages) > 0 {
			output.Detailf("apt: %s", strings.Join(opts.AptPackages, ", "))
		}

		if err := BuildTool(name, image, opts); err != nil {
			return err
		}
	}
	return nil
}
