package docker

import (
	"maps"
	"slices"

	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// UpdateTool rebuilds tool, skipping entirely if it's already up to date and
// opts.Pull is false. When opts.Pull is true the rebuild always runs (even if
// the tool version is unchanged) so a `docker build --pull` can refresh stale
// base image layers - but in that case the tool stage's own cache is left
// alone (no CacheBust) since there's nothing new for it to install. Otherwise
// it recovers base extras, layer versions, and apt packages from the existing
// image's labels when not already set, then delegates to BuildTool with
// CacheBust set so only the tool stage skips cache.
func UpdateTool(tool, image string, opts tools.BuildOptions) error {
	hasUserApt := len(opts.AptPackages) > 0
	userPkgs := opts.AptPackages
	opts.VerifyApt = hasUserApt

	info, err := InspectImage(image)
	upToDate := false
	if err == nil && info != nil {
		upToDate = isUpToDate(tool, info.Version)

		if !opts.NoCache && !opts.Pull && upToDate {
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

	if !opts.NoCache && upToDate {
		opts.CacheBust = ""
	} else if opts.CacheBust == "" {
		opts.CacheBust = NewCacheBust()
	}

	return BuildTool(tool, image, opts)
}

// isUpToDate reports whether tool's installed version matches the latest
// upstream version. Any failure to determine this returns false, so the
// caller falls back to rebuilding.
func isUpToDate(tool, installedLabel string) bool {
	_, newer, ok := LatestToolVersion(tool, installedLabel)
	return ok && !newer
}

// LatestToolVersion fetches the latest version available upstream for tool and
// compares it against installedLabel (an "agentic.tool.version" image label).
// ok is false when there's nothing conclusive to report - treat that as
// inconclusive, not as confirmation of being up to date. When ok is true,
// latest is the normalized latest version, and newer reports whether it
// differs from installedLabel.
func LatestToolVersion(tool, installedLabel string) (latest string, newer bool, ok bool) {
	current := ParseVersion(installedLabel)
	if current == "" {
		return "", false, false
	}

	fetch := tools.Configs[tool].Build.LatestVersion
	if fetch == nil {
		return "", false, false
	}

	raw, err := fetch()
	if err != nil {
		return "", false, false
	}

	latest = ParseVersion(raw)
	return latest, latest != current, true
}

// mergeVersions combines recovered per-layer versions with user overrides,
// with overrides winning.
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
