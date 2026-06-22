package docker

import (
	"maps"
	"slices"

	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// UpdateTool runs a build update for a tool.
// It first checks whether a newer version is available upstream; if the
// installed version already matches, the rebuild is skipped entirely so
// `agentic update` is fast when there is nothing to do. Otherwise it recovers
// the base extras, layer versions, and apt packages from the existing image's
// labels when not already set (so updates preserve the original build
// configuration and the regenerated base/extra stages stay cache-hits), then
// delegates to BuildTool with a CacheBust value set so only the tool stage skips cache.
func UpdateTool(tool, image string, opts tools.BuildOptions) error {
	hasUserApt := len(opts.AptPackages) > 0
	userPkgs := opts.AptPackages
	opts.VerifyApt = hasUserApt

	info, err := InspectImage(image)
	if err == nil && info != nil {
		if !opts.NoCache && isUpToDate(tool, info.Version) {
			return nil
		}

		if len(opts.BaseOverride) == 0 && info.Base != "" {
			opts.BaseOverride = RecoverExtras(info.Base)
		}

		if info.VersionArgs != "" {
			opts.Versions = mergeVersions(RecoverVersionArgs(info.VersionArgs), opts.Versions)
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

// isUpToDate reports whether tool's installed version already matches the
// latest version available upstream. Any failure to determine this (unknown
// installed version, no upstream check configured, or the upstream request
// failing) returns false so the caller falls back to rebuilding as before -
// a missed skip costs a rebuild, a wrong skip would leave the tool stale.
func isUpToDate(tool, installedLabel string) bool {
	current := ParseVersion(installedLabel)
	if current == "" {
		return false
	}

	fetch := tools.Configs[tool].Build.LatestVersion
	if fetch == nil {
		return false
	}

	latest, err := fetch()
	if err != nil {
		return false
	}

	return ParseVersion(latest) == current
}

// mergeVersions combines the recovered per-layer versions with any user-specified
// overrides, with overrides winning - so explicit --node/--java/etc flags (or RC/env
// settings) still take precedence over whatever the original image was built with.
func mergeVersions(recovered, overrides map[string]string) map[string]string {
	merged := make(map[string]string, len(recovered)+len(overrides))
	maps.Copy(merged, recovered)

	for name, ver := range overrides {
		if ver != "" {
			merged[name] = ver
		}
	}

	return merged
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
