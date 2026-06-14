package docker

import (
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/tools"
)

const (
	// LabelCLIVersion records the agentic CLI version (CLIVersion) that built the image.
	LabelCLIVersion = "agentic.version"

	// LabelNamespace records the namespace the image belongs to, recovered from
	// the image name at stamp time. Used to filter images by namespace.
	LabelNamespace = "agentic.namespace"

	// LabelBase records the observed extra-layer versions actually detected inside
	// the built image (see collectBaseLabel). This is what `agentic inspect` shows
	// the user - "what is this image?".
	LabelBase = "agentic.base"

	// LabelVersionArgs records the requested base composition: the exact ARG
	// defaults used to generate the Dockerfile (see buildVersionArgsLabel), which
	// may differ from the detected versions in LabelBase (e.g. requested "17" vs
	// detected "21.0.1"). `agentic update` replays these verbatim (see
	// RecoverVersionArgs) so the base/extra stages stay cache-hits across rebuilds -
	// "how do I rebuild this image identically?".
	LabelVersionArgs = "agentic.version-args"

	// LabelApt records the comma-separated list of apt packages installed in the
	// image (recovered verbatim by RecoverApt so `agentic update` can merge in any
	// newly requested packages without dropping previously installed ones).
	LabelApt = "agentic.apt"

	// LabelTool records the name of the tool baked into the image (e.g. "claude").
	// Used to filter images by tool.
	LabelTool = "agentic.tool"

	// LabelToolVersion records the detected version of the tool itself, read from
	// the image by running its version script (see runVersionScript).
	LabelToolVersion = "agentic.tool.version"

	// LabelBuilt records the UTC timestamp at which the image was built.
	LabelBuilt = "agentic.built"

	// LabelProject marks every docker resource (image, container, volume) created
	// by agentic, paired with LabelProjectVal. Used to scope cleanup and listing
	// to agentic-managed resources only.
	LabelProject = "project"

	LabelProjectVal = "agentic-cli"
)

// CLIVersion is the agentic CLI version stamped onto built images via the
// agentic.version label. Set from cmd.Version at startup.
var CLIVersion = "dev"

// RecoverExtras parses an agentic.base label and returns the extra layer names as a slice.
// e.g. "node@24.2.0,java@21.0.1" → ["node", "java"]
func RecoverExtras(baseLabel string) []string {
	var extras []string

	for part := range strings.SplitSeq(baseLabel, ",") {
		name, _, _ := strings.Cut(part, "@")
		if name == "" {
			continue
		}
		extras = append(extras, name)
	}

	return extras
}

// RecoverVersionArgs parses an agentic.version-args label into a layer name → version map,
// suitable for merging into BuildOptions.Versions so `agentic update` regenerates
// the same ARG defaults the image was originally built with (and so its base/extra
// stages stay cache-hits - only the tool stage gets busted).
// e.g. "node@24,java@17" → {"node": "24", "java": "17"}
func RecoverVersionArgs(versionArgsLabel string) map[string]string {
	versions := make(map[string]string)

	for part := range strings.SplitSeq(versionArgsLabel, ",") {
		name, ver, ok := strings.Cut(part, "@")
		if ok && name != "" && ver != "" {
			versions[name] = ver
		}
	}

	return versions
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

// buildBaseLabel constructs the agentic.base label value from the extra layers
// and their detected versions.
func buildBaseLabel(extras []string, extraVersions map[string]string) string {
	var parts []string
	for _, extra := range extras {
		part := extra
		if ver := extraVersions[extra]; ver != "" {
			part += "@" + ver
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ",")
}

// buildVersionArgsLabel constructs the agentic.version-args label value from the resolved
// version for each layer - the explicit override if one was given, otherwise the
// embedded default - so the exact ARG default baked into the Dockerfile is recorded
// and can be replayed verbatim by `agentic update` (see RecoverVersionArgs).
func buildVersionArgsLabel(layers []string, overrides map[string]string) string {
	var parts []string

	for _, layer := range layers {
		ver := overrides[layer]
		if ver == "" {
			ver = tools.DefaultVersions.ForLayer(layer)
		}
		if ver != "" {
			parts = append(parts, layer+"@"+ver)
		}
	}

	return strings.Join(parts, ",")
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
