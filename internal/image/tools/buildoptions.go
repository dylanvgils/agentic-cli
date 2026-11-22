package tools

import (
	"maps"

	"github.com/dylanvgils/agentic-cli/internal/config/rc"
)

// BuildFlags carries the flag-derived values BuildOptionsFor needs.
type BuildFlags struct {
	Bases               []string
	BasesExact          *[]string // non-nil (even &[]string{}) triggers exact mode, overriding rc.Build.Bases entirely
	VersionOverrides    map[string]string
	AptPackages         []string
	AptPackagesExact    *[]string // non-nil (even &[]string{}) triggers exact mode, overriding rc.Build.AptPackages entirely
	NoCache             bool
	Pull                bool
	CacheFrom           []string
	CacheTo             []string
	Registry            string
	SkipInstallChecksum bool
}

// BuildOptionsFor merges in with rc into the effective BuildOptions.
func BuildOptionsFor(in BuildFlags, agenticRC *rc.AgenticRC) BuildOptions {
	opts := BuildOptions{}

	if in.BasesExact != nil {
		opts.BaseOverride = basesExact(*in.BasesExact)
		opts.BaseExact = true
	} else {
		opts.BaseOverride = bases(in.Bases, agenticRC)
	}

	opts.NoCache = in.NoCache
	opts.Pull = in.Pull
	opts.CacheFrom = in.CacheFrom
	opts.CacheTo = in.CacheTo
	opts.Versions = versions(in.VersionOverrides, agenticRC)

	if in.AptPackagesExact != nil {
		opts.AptPackages = aptPackagesExact(*in.AptPackagesExact)
		opts.AptExact = true
	} else {
		opts.AptPackages = aptPackages(in.AptPackages, agenticRC)
	}
	opts.VerifyApt = len(opts.AptPackages) > 0
	opts.Registry = in.Registry
	opts.CustomInstalls = agenticRC.Build.CustomInstalls
	opts.SkipInstallChecksum = in.SkipInstallChecksum

	return opts
}

// UpdateOptionsFor is BuildOptionsFor for `agentic update`: config bases and apt packages never override what an image was built with,
// so they only apply when baseSet or aptSet says the matching --base/--base-exact or --apt/--apt-exact flag was given.
// Config versions never apply, only --<layer> flags; custom installs are recovered from the image by the update usecase.
func UpdateOptionsFor(in BuildFlags, baseSet, aptSet bool, agenticRC *rc.AgenticRC) BuildOptions {
	opts := BuildOptionsFor(in, agenticRC)
	opts.Versions = versions(in.VersionOverrides, &rc.AgenticRC{})

	if !baseSet {
		opts.BaseOverride = nil
	}
	if !aptSet {
		opts.AptPackages = nil
	}

	return opts
}

// bases merges extra base layers from the project config with flagBases.
func bases(flagBases []string, agenticRC *rc.AgenticRC) []string {
	return SortExtras(MergePackages(agenticRC.Build.Bases, flagBases))
}

// basesExact returns flagBases sorted by canonical order, ignoring rc.Build.Bases entirely.
func basesExact(flagBases []string) []string {
	return SortExtras(flagBases)
}

// versions builds the per-layer version map with RC values as defaults, overridden by flagOverrides.
func versions(flagOverrides map[string]string, agenticRC *rc.AgenticRC) map[string]string {
	merged := make(map[string]string, len(agenticRC.Build.Versions))
	maps.Copy(merged, agenticRC.Build.Versions)

	for name, v := range flagOverrides {
		if v != "" {
			merged[name] = v
		}
	}
	return merged
}

// aptPackages merges apt packages from the project config with flagPkgs.
func aptPackages(flagPkgs []string, agenticRC *rc.AgenticRC) []string {
	return MergePackages(agenticRC.Build.AptPackages, flagPkgs)
}

// aptPackagesExact deduplicates flagPkgs while preserving order, ignoring rc.Build.AptPackages entirely.
func aptPackagesExact(flagPkgs []string) []string {
	return MergePackages(nil, flagPkgs)
}
