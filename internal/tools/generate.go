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
	stages := []dockerfile.Stage{NodeStage(opts.NodeVersion)}
	prev := "base"

	for _, extra := range extras {
		ver := opts.Versions[extra]
		stage, err := ExtraStage(extra, prev, ver)
		if err != nil {
			return nil, err
		}
		stages = append(stages, stage)
		prev = extra
	}

	cfg, ok := Configs[tool]
	if !ok {
		return nil, fmt.Errorf("unknown tool %q", tool)
	}
	stages = append(stages, cfg.Build.Stage(prev))

	return stages, nil
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
