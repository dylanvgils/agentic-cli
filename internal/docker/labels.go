package docker

import (
	"strings"
	"time"
)

const (
	LabelToolVersion = "agentic.tool.version"
	LabelBase        = "agentic.base"
	LabelBuilt       = "agentic.built"
	LabelProject     = "project"
	LabelProjectVal  = "agentic-cli"
)

// RecoverExtras parses an agentic.base label and returns the non-node extras as a
// comma-separated string suitable for BuildOptions.BaseOverride.
// e.g. "node@24.2.0,java@21.0.1" → "java"
func RecoverExtras(baseLabel string) string {
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

// label builds a --label=key=value Docker flag.
func label(key, value string) string {
	return arg("label", key+"="+value)
}

// buildBaseLabel constructs the agentic.base label value from the node version
// and any extra layers with their detected versions.
func buildBaseLabel(nodeVer string, extras []string, extraVersions map[string]string) string {
	var label strings.Builder
	label.WriteString("node")
	if nodeVer != "" {
		label.WriteString("@" + nodeVer)
	}

	for _, extra := range extras {
		label.WriteString("," + extra)
		if ver := extraVersions[extra]; ver != "" {
			label.WriteString("@" + ver)
		}
	}

	return label.String()
}

// buildBuiltLabel returns the current UTC time formatted as the agentic.built label value.
func buildBuiltLabel() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}
