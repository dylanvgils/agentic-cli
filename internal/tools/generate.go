package tools

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/dockerfile"
)

// BuildOptions controls how a tool image is built.
type BuildOptions struct {
	BaseOverride []string          // overrides the tool's default base extras
	NoCache      bool              // disable layer cache for all steps
	CacheBust    string            // non-empty to bust the tool stage's cache via its CACHEBUST build arg (used by update)
	Versions     map[string]string // layer name → version override, e.g. {"node": "22", "java": "21"}
	AptPackages  []string          // additional apt packages to install in the base stage
	VerifyApt    bool              // run pre-build apt-cache check for AptPackages
	Registry     string            // registry prefix for base images (e.g. "myregistry.example.com")
}

// GenerateDockerfile returns the Dockerfile content for the named tool without building it.
func GenerateDockerfile(tool string, opts BuildOptions) (string, error) {
	stages, err := composeStages(tool, opts.BaseOverride, opts)
	if err != nil {
		return "", err
	}
	return dockerfile.File{Stages: stages}.Render(), nil
}

// ParseExtras splits a comma-separated base override string into individual extra names,
// returned in canonical knownExtras order so the generated Dockerfile is deterministic
// and Docker layer caching is not invalidated by flag reordering.
func ParseExtras(base string) []string {
	var extras []string
	for extra := range strings.SplitSeq(base, ",") {
		if extra = strings.TrimSpace(extra); extra != "" {
			extras = append(extras, extra)
		}
	}

	return sortByKnownExtras(extras)
}

// composeStages assembles the full list of Dockerfile stages: node base + requested extras + tool.
func composeStages(tool string, extras []string, opts BuildOptions) ([]dockerfile.Stage, error) {
	pkgs := collectPackages(extras, opts.AptPackages)
	base := baseStage(opts.Versions[BaseLayer], opts.Registry, pkgs)

	extraList, prev, err := buildExtraStages(extras, "base", opts.Versions)
	if err != nil {
		return nil, err
	}

	toolStage, err := resolveToolStage(tool, prev)
	if err != nil {
		return nil, err
	}

	stages := []dockerfile.Stage{base}
	stages = append(stages, extraList...)
	stages = append(stages, toolStage)
	return stages, nil
}

// buildExtraStages chains extra stages (e.g. java, dotnet, go), each building FROM the previous.
// Returns the assembled stages and the name of the final stage in the chain.
func buildExtraStages(extras []string, prevStage string, versions map[string]string) ([]dockerfile.Stage, string, error) {
	var stages []dockerfile.Stage
	prev := prevStage

	for _, extra := range extras {
		ver := versions[extra]
		stage, err := extraStage(extra, prev, ver)
		if err != nil {
			return nil, "", err
		}
		stages = append(stages, stage)
		prev = extra
	}

	return stages, prev, nil
}

// resolveToolStage looks up the tool config and returns its Dockerfile stage,
// with cache-busting instructions prepended right after FROM so a changed
// CACHEBUST build arg invalidates the cache for the entire tool stage.
func resolveToolStage(tool, prevStage string) (dockerfile.Stage, error) {
	cfg, ok := Configs[tool]
	if !ok {
		return dockerfile.Stage{}, fmt.Errorf("unknown tool %q", tool)
	}

	stage := cfg.Build.Stage(prevStage)
	stage.Instructions = append(cacheBustInstructions(), stage.Instructions...)
	return stage, nil
}

// SortExtras returns a copy of extras sorted by canonical knownExtras order.
func SortExtras(extras []string) []string {
	return sortByKnownExtras(extras)
}

// sortByKnownExtras returns a copy of extras sorted by their position in knownExtras.
func sortByKnownExtras(extras []string) []string {
	sorted := slices.Clone(extras)
	slices.SortFunc(sorted, func(a, b string) int {
		return cmp.Compare(slices.Index(knownExtras, a), slices.Index(knownExtras, b))
	})
	return sorted
}
