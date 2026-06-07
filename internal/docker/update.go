package docker

import (
	"slices"

	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// UpdateTool runs a build update for a tool.
// It recovers the base extras and apt packages from the existing image's labels when
// not already set (so updates preserve the original build configuration),
// then delegates to BuildTool with a CacheBust value set so only the tool stage skips cache.
func UpdateTool(tool, image string, opts tools.BuildOptions) error {
	hasUserApt := len(opts.AptPackages) > 0
	userPkgs := opts.AptPackages
	opts.VerifyApt = hasUserApt

	info, err := InspectImage(image)
	if err == nil && info != nil {
		if opts.BaseOverride == "" && info.Base != "" {
			opts.BaseOverride = RecoverExtras(info.Base)
		}

		if info.Apt != "" {
			recoveredPkgs := RecoverApt(info.Apt)
			opts.AptPackages = tools.MergePackages(recoveredPkgs, opts.AptPackages)
			opts.VerifyApt = hasUserApt && hasNewAptPackages(userPkgs, recoveredPkgs)
		}
	}

	if opts.CacheBust == "" {
		opts.CacheBust = NewCacheBust()
	}

	return BuildTool(tool, image, opts)
}

// hasNewAptPackages returns true if any package in requested is not present in existing.
func hasNewAptPackages(requested, existing []string) bool {
	for _, pkg := range requested {
		if !slices.Contains(existing, pkg) {
			return true
		}
	}
	return false
}

