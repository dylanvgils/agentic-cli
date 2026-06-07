package docker

import (
	"strings"
	"time"
)

const (
	LabelCLIVersion  = "agentic.version"
	LabelNamespace   = "agentic.namespace"
	LabelBase        = "agentic.base"
	LabelApt         = "agentic.apt"
	LabelTool        = "agentic.tool"
	LabelToolVersion = "agentic.tool.version"
	LabelBuilt       = "agentic.built"
	LabelProject     = "project"

	LabelProjectVal = "agentic-cli"
)

// CLIVersion is the agentic CLI version stamped onto built images via the
// agentic.version label. Set from cmd.Version at startup.
var CLIVersion = "dev"

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

// RecoverApt parses an agentic.apt label value into a slice of package names.
func RecoverApt(aptLabel string) []string {
	var pkgs []string
	for pkg := range strings.SplitSeq(aptLabel, ",") {
		if pkg = strings.TrimSpace(pkg); pkg != "" {
			pkgs = append(pkgs, pkg)
		}
	}
	return pkgs
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

// NewCacheBust returns a value that changes between `agentic update` invocations
// but can be reused across every target built within a single invocation, so
// Docker can still serve cached tool-stage layers when the same tool is rebuilt
// for multiple namespaces in one run.
func NewCacheBust() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
