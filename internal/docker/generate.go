package docker

import (
	"fmt"
	"strings"

	"github.com/dylanvgils/agentic-cli/internal/dockerfile"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

// GenerateDockerfile returns the Dockerfile content for the named tool without building it.
func GenerateDockerfile(tool string, opts BuildOptions) (string, error) {
	stages, err := composeStages(tool, parseExtras(opts.BaseOverride), opts)
	if err != nil {
		return "", err
	}
	return dockerfile.File{Stages: stages}.Render(), nil
}

// composeStages assembles the full list of Dockerfile stages: node base + requested extras + tool.
func composeStages(tool string, extras []string, opts BuildOptions) ([]dockerfile.Stage, error) {
	stages := []dockerfile.Stage{tools.NodeStage(opts.NodeVersion)}
	prev := "base"

	for _, extra := range extras {
		ver := opts.Versions[extra]
		stage, err := tools.ExtraStage(extra, prev, ver)
		if err != nil {
			return nil, err
		}
		stages = append(stages, stage)
		prev = extra
	}

	cfg, ok := tools.Configs[tool]
	if !ok {
		return nil, fmt.Errorf("unknown tool %q", tool)
	}
	stages = append(stages, cfg.Stage(prev))

	return stages, nil
}

func parseExtras(base string) []string {
	var extras []string
	for extra := range strings.SplitSeq(base, ",") {
		if extra = strings.TrimSpace(extra); extra != "" {
			extras = append(extras, extra)
		}
	}
	return extras
}
