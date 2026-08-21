package docker

import (
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

const (
	// -- Identity: which image this is --

	// LabelNamespace records the namespace the image belongs to, recovered from
	// the image name at stamp time. Used to filter images by namespace.
	LabelNamespace = "agentic.namespace"

	// LabelTool records the name of the tool baked into the image (e.g. "claude").
	// Used to filter images by tool.
	LabelTool = "agentic.tool"

	// -- Build provenance: what went into the image and how to rebuild it --

	// LabelCLIVersion records the agentic CLI version (buildinfo.Version) that
	// built the image.
	LabelCLIVersion = "agentic.version"

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

	// LabelCustomInstalls records the comma-separated list of custom_installs
	// names baked into the image, purely for `agentic inspect` display -
	// unlike LabelApt there is no Recover* for it, since custom_installs always
	// reads fresh from the current .agenticrc.toml on every build.
	LabelCustomInstalls = "agentic.custom-installs"

	// LabelToolVersion records the detected version of the tool itself, read from
	// the image by running its version script (see runVersionScript).
	LabelToolVersion = "agentic.tool.version"

	// -- Timestamps --

	// LabelBuilt records the UTC timestamp at which the image was built.
	LabelBuilt = "agentic.built"

	// LabelPulled records the UTC timestamp at which `docker build --pull` was
	// last actually run for this image, so `agentic update` can throttle how
	// often it automatically re-checks the registry for fresher base images.
	LabelPulled = "agentic.pulled"

	// -- Cache --

	// LabelCacheBust records the CACHEBUST build-arg baked into the tool stage,
	// reused verbatim on cache-hit rebuilds so Docker resolves the same layer.
	LabelCacheBust = "agentic.cachebust"

	// -- Project marker --

	// LabelProject marks every docker resource (image, container, volume) created
	// by agentic, paired with LabelProjectVal. Used to scope cleanup and listing
	// to agentic-managed resources only.
	LabelProject = "project"

	LabelProjectVal = "agentic-cli"
)

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

// PullIsFresh reports whether pulledLabel (an agentic.pulled label value)
// shows a pull within interval, so `agentic update` can skip a redundant
// automatic --pull. An empty or unparseable label is treated as not fresh, so
// the caller falls back to pulling.
func PullIsFresh(pulledLabel string, interval time.Duration) bool {
	t, ok := parseLabelTime(pulledLabel)
	return ok && time.Since(t) < interval
}

// NewCacheBust returns a value that changes between `agentic update` invocations
// but can be reused across every target built within a single invocation, so
// Docker can still serve cached tool-stage layers when the same tool is rebuilt
// for multiple namespaces in one run.
func NewCacheBust() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
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

// formatLabelTime formats t as the UTC timestamp string used by agentic's
// timestamp-valued labels (agentic.built, agentic.pulled).
func formatLabelTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

// parseLabelTime parses a timestamp previously formatted by formatLabelTime.
func parseLabelTime(s string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02T15:04:05Z", s)
	return t, err == nil
}

// buildBuiltLabel returns the current UTC time formatted as the agentic.built label value.
func buildBuiltLabel() string {
	return formatLabelTime(time.Now())
}

// buildPulledLabel returns the current UTC time formatted as the agentic.pulled label value.
func buildPulledLabel() string {
	return formatLabelTime(time.Now())
}

// imageLabelPairs lists every agentic image label paired with its value on info.
func imageLabelPairs(info ImageInfo) []struct{ key, value string } {
	return []struct{ key, value string }{
		{LabelNamespace, info.Namespace},
		{LabelTool, info.Tool},
		{LabelToolVersion, info.Version},
		{LabelBase, info.Base},
		{LabelVersionArgs, info.VersionArgs},
		{LabelApt, info.Apt},
		{LabelCustomInstalls, info.CustomInstalls},
		{LabelBuilt, info.Built},
		{LabelPulled, info.Pulled},
		{LabelCLIVersion, info.CLIVersion},
		{LabelCacheBust, info.CacheBust},
	}
}

// stampLabels relabels image with LabelProject plus every non-empty label in info.
func stampLabels(image string, info ImageInfo) {
	args := []string{"build", label(LabelProject, LabelProjectVal)}

	for _, p := range imageLabelPairs(info) {
		if p.value != "" {
			args = append(args, label(p.key, p.value))
		}
	}

	args = append(args, arg("tag", image), "-")
	_, _ = dockerRunStdin(strings.NewReader("FROM "+image+"\n"), args...)
}

// stampImageLabels detects base and tool versions from the built image and stamps them via stampLabels.
// cacheBust becomes LabelCacheBust, so a later pull-only rebuild can reuse it verbatim.
func stampImageLabels(image, tool string, extras []string, aptPkgs []string, versions map[string]string, customInstalls []string, cacheBust string) {
	layers := append([]string{tools.BaseLayer}, extras...)

	info := ImageInfo{
		Namespace:      strings.TrimSuffix(image, "-"+tool),
		Tool:           tool,
		Base:           collectBaseLabel(image, extras),
		VersionArgs:    buildVersionArgsLabel(layers, versions),
		Apt:            strings.Join(aptPkgs, ","),
		CustomInstalls: strings.Join(customInstalls, ","),
		Built:          buildBuiltLabel(),
		CLIVersion:     buildinfo.Version,
		CacheBust:      cacheBust,
	}
	info.Version = runVersionScript(image, versionScript(tool))

	stampLabels(image, info)
}
