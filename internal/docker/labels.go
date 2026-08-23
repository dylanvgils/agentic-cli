package docker

import (
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/tools"
)

const (
	// -- Identity: which image this is --

	// LabelNamespace records the namespace an image belongs to, recovered from the image name at stamp time.
	LabelNamespace = "agentic.namespace"

	// LabelTool records the name of the tool baked into the image (e.g. "claude").
	LabelTool = "agentic.tool"

	// -- Build provenance: what went into the image and how to rebuild it --

	// LabelCLIVersion records the agentic CLI version (buildinfo.Version) that built the image.
	LabelCLIVersion = "agentic.version"

	// LabelBase records the extra-layer versions actually detected inside the built image
	// (see collectBaseLabel) - what `agentic inspect` shows as "what is this image?".
	LabelBase = "agentic.base"

	// LabelVersionArgs records the requested ARG defaults used to generate the Dockerfile (see
	// buildVersionArgsLabel), which `agentic update` replays via RecoverVersionArgs to keep
	// base/extra stages cache-hits across rebuilds - "how do I rebuild this identically?".
	LabelVersionArgs = "agentic.version-args"

	// LabelApt records the comma-separated apt packages installed, recovered verbatim by
	// RecoverApt so `agentic update` can merge in new packages without dropping old ones.
	LabelApt = "agentic.apt"

	// LabelCustomInstalls records the comma-separated custom_installs names, for `agentic
	// inspect` display only - always read fresh from .agenticrc.toml on build, unlike LabelApt.
	LabelCustomInstalls = "agentic.custom-installs"

	// LabelToolVersion records the detected tool version, read via its version script (see runVersionScript).
	LabelToolVersion = "agentic.tool.version"

	// -- Timestamps --

	// LabelBuilt records the UTC timestamp at which the image was built.
	LabelBuilt = "agentic.built"

	// LabelPulled records when `docker build --pull` last ran, so `agentic update` can throttle automatic re-pulls.
	LabelPulled = "agentic.pulled"

	// -- Cache --

	// LabelCacheBust records the CACHEBUST build-arg baked into the tool stage, reused verbatim on cache-hit rebuilds.
	LabelCacheBust = "agentic.cachebust"

	// -- Project marker --

	// LabelProject marks every docker resource agentic created, paired with LabelProjectVal, to scope cleanup and listing.
	LabelProject = "project"

	LabelProjectVal = "agentic-cli"
)

// RecoverExtras parses an agentic.base label into extra layer names, e.g. "node@24.2.0,java@21.0.1" -> ["node", "java"].
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

// RecoverVersionArgs parses an agentic.version-args label into a layer name -> version map
// (e.g. "node@24,java@17" -> {"node": "24", "java": "17"}) for merging into BuildOptions.Versions.
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

// RecoveredBaseOverride returns opts.BaseOverride, recovering it from info's agentic.base label unless opts.BaseExact or an override is already set.
func RecoveredBaseOverride(info *ImageInfo, opts tools.BuildOptions) []string {
	if !opts.BaseExact && len(opts.BaseOverride) == 0 && info.Base != "" {
		return RecoverExtras(info.Base)
	}
	return opts.BaseOverride
}

// RecoveredAptPackages merges opts.AptPackages with packages recovered from info's agentic.apt label, unless opts.AptExact; recovered is nil when recovery was skipped.
func RecoveredAptPackages(info *ImageInfo, opts tools.BuildOptions) (merged, recovered []string) {
	if opts.AptExact || info.Apt == "" {
		return opts.AptPackages, nil
	}
	recovered = RecoverApt(info.Apt)
	return tools.MergePackages(recovered, opts.AptPackages), recovered
}

// PullIsFresh reports whether pulledLabel shows a pull within interval; empty or unparseable is treated as not fresh.
func PullIsFresh(pulledLabel string, interval time.Duration) bool {
	t, ok := parseLabelTime(pulledLabel)
	return ok && time.Since(t) < interval
}

// NewCacheBust returns a value that changes between `agentic update` invocations but is shared across targets within one, so Docker can still cache-hit the same tool rebuilt across namespaces.
func NewCacheBust() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// label builds a --label=key=value Docker flag.
func label(key, value string) string {
	return arg("label", key+"="+value)
}

// buildBaseLabel constructs the agentic.base label value from the extra layers and their detected versions.
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

// buildVersionArgsLabel constructs the agentic.version-args label from each layer's resolved
// version (override or embedded default), so `agentic update` can replay it via RecoverVersionArgs.
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

// formatLabelTime formats t as the UTC timestamp string used by agentic's timestamp-valued labels.
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
