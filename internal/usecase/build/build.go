// Package build builds tool images for `agentic build`, printing generated Dockerfiles instead in dry-run mode.
package build

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// BuildTool indirects the docker call so callers can fake it in tests (seam convention, see internal/cli/root.go).
var BuildTool = docker.BuildTool

// DryRun prints the generated Dockerfile for each tool in names instead of building it.
func DryRun(names []string, opts tools.BuildOptions) error {
	for _, name := range names {
		logging.Step(name)
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

// Apply builds each tool image in names under namespace, reporting the base/apt overrides in effect for each.
func Apply(names []string, namespace string, opts tools.BuildOptions) error {
	for _, name := range names {
		image, err := tools.ImageName(name, namespace)
		if err != nil {
			return err
		}

		logging.Step(image)
		if len(opts.BaseOverride) > 0 {
			logging.Detailf("base: %s", strings.Join(opts.BaseOverride, ", "))
		} else if opts.BaseExact {
			logging.Detail("base: (none, exact)")
		}
		if len(opts.AptPackages) > 0 {
			logging.Detailf("apt: %s", strings.Join(opts.AptPackages, ", "))
		} else if opts.AptExact {
			logging.Detail("apt: (none, exact)")
		}

		if err := BuildTool(name, image, opts); err != nil {
			return err
		}
	}
	return nil
}
