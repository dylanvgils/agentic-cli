package docker

import (
	"maps"
	"slices"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// UpdateTool rebuilds tool. If it's already up to date and opts.Pull is
// false, it skips the reinstall but still restamps labels so `agentic
// inspect` stays current; otherwise it recovers base/version/apt labels from
// the existing image and delegates to BuildTool.
func UpdateTool(tool, image string, opts tools.BuildOptions) error {
	hasUserApt := len(opts.AptPackages) > 0
	userPkgs := opts.AptPackages
	opts.VerifyApt = hasUserApt

	info, err := InspectImage(image)
	upToDate := false
	if err == nil && info != nil {
		upToDate = isUpToDate(tool, info.Version)

		if !opts.NoCache && !opts.Pull && upToDate {
			info.Built = buildBuiltLabel()
			info.CLIVersion = buildinfo.Version
			stampLabels(image, *info)
			return nil
		}

		opts.BaseOverride = RecoveredBaseOverride(info, opts)

		if info.VersionArgs != "" {
			opts.Versions = mergeVersions(RecoverVersionArgs(info.VersionArgs), opts.Versions)
		}

		var recoveredPkgs []string
		opts.AptPackages, recoveredPkgs = RecoveredAptPackages(info, opts)
		if recoveredPkgs != nil {
			opts.VerifyApt = hasUserApt && hasNewAptPackages(userPkgs, recoveredPkgs)
		}
	}

	if !opts.NoCache && upToDate {
		// Reuse the existing CacheBust (LabelCacheBust) instead of clearing it,
		// or Docker's cache falls back to a stale unrelated build.
		if info != nil {
			opts.CacheBust = info.CacheBust
		}
	} else if opts.CacheBust == "" {
		opts.CacheBust = NewCacheBust()
	}

	return BuildTool(tool, image, opts)
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

// isUpToDate reports whether tool's installed version matches the latest
// upstream version. Any failure to determine this returns false, so the
// caller falls back to rebuilding.
func isUpToDate(tool, installedLabel string) bool {
	_, newer, ok := LatestToolVersion(tool, installedLabel)
	return ok && !newer
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
