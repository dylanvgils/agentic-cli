package settings

import (
	"maps"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// BuildInput carries the flag-derived values BuildOptions needs.
type BuildInput struct {
	Bases            []string
	VersionOverrides map[string]string
	AptPackages      []string
	NoCache          bool
	Pull             bool
	Registry         string
}

// BuildOptions merges in with rc into the effective tools.BuildOptions.
func BuildOptions(in BuildInput, rc *config.AgenticRC) tools.BuildOptions {
	opts := tools.BuildOptions{}

	opts.BaseOverride = Bases(in.Bases, rc)
	opts.NoCache = in.NoCache
	opts.Pull = in.Pull
	opts.Versions = Versions(in.VersionOverrides, rc)
	opts.AptPackages = AptPackages(in.AptPackages, rc)
	opts.VerifyApt = len(opts.AptPackages) > 0
	opts.Registry = in.Registry
	opts.CustomInstalls = rc.Build.CustomInstalls

	return opts
}

// Bases merges extra base layers from the project config with flagBases.
func Bases(flagBases []string, rc *config.AgenticRC) []string {
	return tools.SortExtras(tools.MergePackages(rc.Build.Bases, flagBases))
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
