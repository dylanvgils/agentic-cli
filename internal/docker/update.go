package docker

import "strings"

// UpdateTool runs a build update for a tool.
// It recovers the base extras from the existing image's agentic.base label when
// BaseOverride is not set (so updates preserve the original --base configuration),
// then delegates to BuildTool with NoCacheTool enabled so only the tool step skips cache.
func UpdateTool(toolDir, image, versionCmd, repoRoot string, opts BuildOptions) error {
	if opts.BaseOverride == "" {
		info, err := InspectImage(image)
		if err == nil && info != nil && info.Base != "" {
			opts.BaseOverride = recoverExtras(info.Base)
		}
	}

	opts.NoCacheTool = true
	return BuildTool(toolDir, image, versionCmd, repoRoot, opts)
}

// recoverExtras parses an agentic.base label and returns the non-node extras as a
// comma-separated string suitable for BuildOptions.BaseOverride.
// e.g. "node@24.2.0,java@21.0.1" → "java"
func recoverExtras(baseLabel string) string {
	var extras []string

	for part := range strings.SplitSeq(baseLabel, ",") {
		name, _, _ := strings.Cut(part, "@")
		if name == "" || name == "node" {
			continue
		}
		extras = append(extras, name)
	}

	return strings.Join(extras, ",")
}
