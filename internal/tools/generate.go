package tools

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/dockerfile"
)

// BuildOptions controls how a tool image is built.
type BuildOptions struct {
	BaseOverride string            // overrides the tool's default base extras (comma-separated)
	NoCache      bool              // disable layer cache for all steps
	NoCacheTool  bool              // disable layer cache for the tool step only (used by update)
	NodeVersion  string            // override Node.js version
	Versions     map[string]string // extra name → version override, e.g. {"java": "21"}
}

// GenerateDockerfile returns the Dockerfile content for the named tool without building it.
func GenerateDockerfile(tool string, opts BuildOptions) (string, error) {
	stages, err := composeStages(tool, ParseExtras(opts.BaseOverride), opts)
	if err != nil {
		return "", err
	}
	return dockerfile.File{Stages: stages}.Render(), nil
}

// composeStages assembles the full list of Dockerfile stages: node base + requested extras + tool.
func composeStages(tool string, extras []string, opts BuildOptions) ([]dockerfile.Stage, error) {
	base := NodeStage(opts.NodeVersion)

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
		stage, err := ExtraStage(extra, prev, ver)
		if err != nil {
			return nil, "", err
		}
		stages = append(stages, stage)
		prev = extra
	}

	return stages, prev, nil
}

// resolveToolStage looks up the tool config and returns its Dockerfile stage.
func resolveToolStage(tool, prevStage string) (dockerfile.Stage, error) {
	cfg, ok := Configs[tool]
	if !ok {
		return dockerfile.Stage{}, fmt.Errorf("unknown tool %q", tool)
	}
	return cfg.Build.Stage(prevStage), nil
}

// ParseExtras splits a comma-separated base override string into individual extra names.
func ParseExtras(base string) []string {
	var extras []string
	for extra := range strings.SplitSeq(base, ",") {
		if extra = strings.TrimSpace(extra); extra != "" {
			extras = append(extras, extra)
		}
	}
	return extras
}
