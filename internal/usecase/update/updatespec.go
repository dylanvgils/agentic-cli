// Package update resolves and applies `agentic update` targets - which
// tool images to rebuild, with which recovered build options, and whether an
// automatic --pull should be throttled.
package update

import (
	"fmt"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/output"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// autoPullInterval bounds how often an automatic --pull is allowed to hit
// the registry again for the same image.
const autoPullInterval = 24 * time.Hour

// Target is one tool image to rebuild, with its resolved build options.
type Target struct {
	Name  string
	Image string
	Opts  tools.BuildOptions
}

// Scope describes which tools/images Resolve should target.
type Scope struct {
	// Names is the tool name list to resolve in scoped mode - already
	// expanded from CLI args to every known tool when no tool was given.
	Names   []string
	HasArgs bool
	// FilterTool narrows an --all resolve to one tool; empty means every tool.
	FilterTool string
	Namespace  string
	All        bool
}

// Resolve returns the update targets for scope: either every image found
// across all namespaces (All) or the named tools in a single namespace.
func Resolve(scope Scope, opts tools.BuildOptions, pullExplicit bool) ([]Target, error) {
	if scope.All {
		return resolveAll(scope.FilterTool, opts, pullExplicit)
	}
	return resolveScoped(scope.Names, scope.HasArgs, scope.Namespace, opts, pullExplicit)
}

func resolveAll(filterTool string, opts tools.BuildOptions, pullExplicit bool) ([]Target, error) {
	var filters []docker.ImageFilter
	if filterTool != "" {
		filters = append(filters, docker.ToolFilter(filterTool))
	}

	images, err := ListAllImages(filters...)
	if err != nil {
		return nil, err
	}

	var targets []Target
	for _, info := range images {
		if _, ok := tools.Configs[info.Tool]; !ok {
			continue
		}
		toolOpts := applyPullThrottle(recoverOpts(info, opts), info, pullExplicit)
		targets = append(targets, Target{Name: info.Tool, Image: info.Image, Opts: toolOpts})
	}
	return targets, nil
}

func resolveScoped(names []string, hasArgs bool, namespace string, opts tools.BuildOptions, pullExplicit bool) ([]Target, error) {
	skipUnbuilt := !hasArgs
	var targets []Target

	for _, name := range names {
		image, err := tools.ImageName(name, namespace)
		if err != nil {
			return nil, err
		}

		info, err := InspectImage(image)
		if err != nil {
			return nil, err
		}

		if skipUnbuilt && info == nil {
			output.Stepf("%s (skipped - not built)", image)
			continue
		}

		toolOpts := opts
		if info != nil {
			toolOpts = recoverOpts(info, opts)
		}
		toolOpts = applyPullThrottle(toolOpts, info, pullExplicit)

		targets = append(targets, Target{Name: name, Image: image, Opts: toolOpts})
	}

	return targets, nil
}

// applyPullThrottle leaves opts.Pull untouched when the user explicitly set
// --pull/--pull=false, or when there's no existing image to check. Otherwise
// it disables the automatic pull if the image's agentic.pulled label shows
// a pull already happened within autoPullInterval, so `agentic update` doesn't
// hit the registry on every single run.
func applyPullThrottle(opts tools.BuildOptions, info *docker.ImageInfo, pullExplicit bool) tools.BuildOptions {
	if pullExplicit || info == nil {
		return opts
	}

	if docker.PullIsFresh(info.Pulled, autoPullInterval) {
		opts.Pull = false
	}

	return opts
}

// DryRun prints the generated Dockerfile for tool instead of building it,
// recovering build options from the existing image's labels first.
func DryRun(tool, namespace string, opts tools.BuildOptions) error {
	if tool == "" {
		return fmt.Errorf("--dry-run requires a tool argument")
	}

	image, err := tools.ImageName(tool, namespace)
	if err == nil {
		if info, iErr := InspectImage(image); iErr == nil && info != nil {
			opts = recoverOpts(info, opts)
		}
	}

	output.Step(tool)
	content, err := tools.GenerateDockerfile(tool, opts)
	if err != nil {
		return err
	}

	_, err = fmt.Println(content)
	return err
}

func recoverOpts(info *docker.ImageInfo, opts tools.BuildOptions) tools.BuildOptions {
	if len(opts.BaseOverride) == 0 {
		opts.BaseOverride = docker.RecoverExtras(info.Base)
	}
	if info.Apt != "" {
		recoveredPkgs := docker.RecoverApt(info.Apt)
		opts.AptPackages = tools.MergePackages(recoveredPkgs, opts.AptPackages)
	}
	return opts
}

// ApplyRecovered rebuilds image for tool, reusing the currently installed
// build options where possible. It's the toolupdate.Updater used by the
// auto-update prompt in `agentic run` - unlike `agentic update`'s own flow,
// it never exits the process, since runTool must still start the tool
// container afterward whether or not the update succeeded. Unlike
// BaseOverride/AptPackages, CustomInstalls is always taken from rc, not
// recovered from an image label - see recoverOpts.
func ApplyRecovered(tool, image string, rc *config.AgenticRC) error {
	opts := tools.BuildOptions{CustomInstalls: rc.Build.CustomInstalls}
	if info, err := InspectImage(image); err == nil && info != nil {
		opts = recoverOpts(info, opts)
	}
	return Apply(tool, image, opts)
}

// Apply rebuilds image for tool with opts, reporting the base/apt overrides
// and predicted version change in effect before the build starts.
func Apply(name, image string, opts tools.BuildOptions) error {
	output.Step(image)
	if len(opts.BaseOverride) > 0 {
		output.Detailf("base: %s", strings.Join(opts.BaseOverride, ", "))
	}
	if len(opts.AptPackages) > 0 {
		output.Detailf("apt: %s", strings.Join(opts.AptPackages, ", "))
	}

	reportBeforeUpdate(name, image)

	return UpdateTool(name, image, opts)
}

// reportBeforeUpdate prints the version-change line before the build starts,
// predicted from the installed version and the latest upstream version - the
// same "version: X -> Y" / "version: X (up to date)" wording Apply used to
// print only after the build finished. Nothing prints for an unbuilt image
// or when the upstream check is inconclusive.
func reportBeforeUpdate(name, image string) {
	info, err := InspectImage(image)
	if err != nil || info == nil {
		return
	}

	before := docker.ParseVersion(info.Version)
	latest, newer, ok := LatestToolVersion(name, info.Version)
	if !ok {
		return
	}

	if newer {
		reportVersionChange(before, latest)
	} else {
		reportVersionChange(before, before)
	}
}

// reportVersionChange prints before/after as a "version: X -> Y" transition,
// or "version: X (up to date)" when they match. after may be a predicted
// target version rather than a confirmed post-build one.
func reportVersionChange(before, after string) {
	if after == "" {
		return
	}

	if before == "" {
		output.Detailf("version: %s", after)
		return
	}

	if before != after {
		output.Detailf("version: %s -> %s", before, after)
	} else {
		output.Detailf("version: %s (up to date)", after)
	}
}
