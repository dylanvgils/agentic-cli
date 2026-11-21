package precedence

import (
	"maps"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/image/tools"
)

// BuildInput carries the flag-derived values BuildOptions needs.
type BuildInput struct {
	Bases               []string
	BasesExact          *[]string // non-nil (even &[]string{}) triggers exact mode, overriding rc.Build.Bases entirely
	VersionOverrides    map[string]string
	AptPackages         []string
	AptPackagesExact    *[]string // non-nil (even &[]string{}) triggers exact mode, overriding rc.Build.AptPackages entirely
	NoCache             bool
	Pull                bool
	Registry            string
	SkipInstallChecksum bool
}

// BuildOptions merges in with rc into the effective tools.BuildOptions.
func BuildOptions(in BuildInput, rc *config.AgenticRC) tools.BuildOptions {
	opts := tools.BuildOptions{}

	if in.BasesExact != nil {
		opts.BaseOverride = basesExact(*in.BasesExact)
		opts.BaseExact = true
	} else {
		opts.BaseOverride = bases(in.Bases, rc)
	}

	opts.NoCache = in.NoCache
	opts.Pull = in.Pull
	opts.Versions = versions(in.VersionOverrides, rc)

	if in.AptPackagesExact != nil {
		opts.AptPackages = aptPackagesExact(*in.AptPackagesExact)
		opts.AptExact = true
	} else {
		opts.AptPackages = aptPackages(in.AptPackages, rc)
	}
	opts.VerifyApt = len(opts.AptPackages) > 0
	opts.Registry = in.Registry
	opts.CustomInstalls = rc.Build.CustomInstalls
	opts.SkipInstallChecksum = in.SkipInstallChecksum

	return opts
}

// UpdateOptions is BuildOptions for `agentic update`: config bases and apt packages never override what an image was built with,
// so they only apply when baseSet or aptSet says the matching --base/--base-exact or --apt/--apt-exact flag was given.
// Config versions never apply, only --<layer> flags; custom installs are recovered from the image by the update usecase.
func UpdateOptions(in BuildInput, baseSet, aptSet bool, rc *config.AgenticRC) tools.BuildOptions {
	opts := BuildOptions(in, rc)
	opts.Versions = versions(in.VersionOverrides, &config.AgenticRC{})

	if !baseSet {
		opts.BaseOverride = nil
	}
	if !aptSet {
		opts.AptPackages = nil
	}

	return opts
}

// bases merges extra base layers from the project config with flagBases.
func bases(flagBases []string, rc *config.AgenticRC) []string {
	return tools.SortExtras(tools.MergePackages(rc.Build.Bases, flagBases))
}

// basesExact returns flagBases sorted by canonical order, ignoring rc.Build.Bases entirely.
func basesExact(flagBases []string) []string {
	return tools.SortExtras(flagBases)
}

// versions builds the per-layer version map with RC values as defaults, overridden by flagOverrides.
func versions(flagOverrides map[string]string, rc *config.AgenticRC) map[string]string {
	merged := make(map[string]string, len(rc.Build.Versions))
	maps.Copy(merged, rc.Build.Versions)

	for name, v := range flagOverrides {
		if v != "" {
			merged[name] = v
		}
	}
	return merged
}

// aptPackages merges apt packages from the project config with flagPkgs.
func aptPackages(flagPkgs []string, rc *config.AgenticRC) []string {
	return tools.MergePackages(rc.Build.AptPackages, flagPkgs)
}

// aptPackagesExact deduplicates flagPkgs while preserving order, ignoring rc.Build.AptPackages entirely.
func aptPackagesExact(flagPkgs []string) []string {
	return tools.MergePackages(nil, flagPkgs)
}
