package resolve

import (
	"maps"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// BuildInput carries the flag-derived values BuildOptions needs.
type BuildInput struct {
	Bases            []string
	BasesExact       *[]string // non-nil (even &[]string{}) triggers exact mode, overriding rc.Build.Bases entirely
	VersionOverrides map[string]string
	AptPackages      []string
	AptPackagesExact *[]string // non-nil (even &[]string{}) triggers exact mode, overriding rc.Build.AptPackages entirely
	NoCache          bool
	Pull             bool
	Registry         string
}

// BuildOptions merges in with rc into the effective tools.BuildOptions.
func BuildOptions(in BuildInput, rc *config.AgenticRC) tools.BuildOptions {
	opts := tools.BuildOptions{}

	if in.BasesExact != nil {
		opts.BaseOverride = BasesExact(*in.BasesExact)
		opts.BaseExact = true
	} else {
		opts.BaseOverride = Bases(in.Bases, rc)
	}

	opts.NoCache = in.NoCache
	opts.Pull = in.Pull
	opts.Versions = Versions(in.VersionOverrides, rc)

	if in.AptPackagesExact != nil {
		opts.AptPackages = AptPackagesExact(*in.AptPackagesExact)
		opts.AptExact = true
	} else {
		opts.AptPackages = AptPackages(in.AptPackages, rc)
	}
	opts.VerifyApt = len(opts.AptPackages) > 0
	opts.Registry = in.Registry
	opts.CustomInstalls = rc.Build.CustomInstalls

	return opts
}

// Bases merges extra base layers from the project config with flagBases.
func Bases(flagBases []string, rc *config.AgenticRC) []string {
	return tools.SortExtras(tools.MergePackages(rc.Build.Bases, flagBases))
}

// BasesExact returns flagBases sorted by canonical order, ignoring rc.Build.Bases entirely.
func BasesExact(flagBases []string) []string {
	return tools.SortExtras(flagBases)
}

// Versions builds the per-layer version map with RC values as defaults, overridden by flagOverrides.
func Versions(flagOverrides map[string]string, rc *config.AgenticRC) map[string]string {
	versions := make(map[string]string, len(rc.Build.Versions))
	maps.Copy(versions, rc.Build.Versions)

	for name, v := range flagOverrides {
		if v != "" {
			versions[name] = v
		}
	}
	return versions
}

// AptPackages merges apt packages from the project config with flagPkgs.
func AptPackages(flagPkgs []string, rc *config.AgenticRC) []string {
	return tools.MergePackages(rc.Build.AptPackages, flagPkgs)
}

// AptPackagesExact deduplicates flagPkgs while preserving order, ignoring rc.Build.AptPackages entirely.
func AptPackagesExact(flagPkgs []string) []string {
	return tools.MergePackages(nil, flagPkgs)
}
